// Package audio captures the microphone and the system's audio output and
// encodes them into one AAC (.m4a) file as they arrive, instead of buffering
// the whole recording in memory.
//
// Capturing is macOS only (ScreenCaptureKit, macOS 15+); on other platforms
// Available reports that no capture is possible.
package audio

import (
	"os"
	"sync"
	"time"
)

// Source is an audio input the Recorder can capture.
type Source string

// The sources the interface offers.
const (
	// SourceMic is the default microphone.
	SourceMic Source = "mic"
	// SourceSystem is the system's audio output: whatever the speakers or
	// headphones are playing.
	SourceSystem Source = "system"
)

// State is where the Recorder is in its lifecycle.
type State int

// The states, in order.
const (
	StateIdle State = iota
	StateStarting
	StateRecording
	StateStopping
	// StatePaused keeps capture running but drops what it delivers, so
	// resuming is instant and cannot fail; the file simply goes on from
	// where it paused.
	StatePaused
)

// The format every source is mixed into before it is encoded: 48 kHz
// stereo, which is what ScreenCaptureKit delivers.
const (
	sampleRate = 48000
	channels   = 2
)

// aacBitRate is the AAC output bit rate, in bits per second.
const aacBitRate = 128000

// chunk is one buffer of interleaved stereo frames a source delivered.
type chunk struct {
	samples []float32 // interleaved stereo frames (L, R, L, R, …)
}

// sourceSamples is one source's buffer on its way to the encode goroutine.
type sourceSamples struct {
	src     Source
	samples []float32
	// flush asks the encode goroutine to write out what it holds, for a
	// pause: the sources' small arrival skew would otherwise be left in the
	// mixer and come out after the pause, ahead of the resumed audio.
	flush bool
}

// encoder streams mixed audio to a file, encoding frames as they arrive
// instead of buffering the whole recording in memory.
type encoder interface {
	write(samples []float32) error
	close() error
}

// Recorder captures the chosen sources and encodes them into an AAC file as
// it records. A single goroutine owns the mixer and encoder, fed by the
// audio callbacks through the samples channel; mu guards the state the UI
// thread reads.
type Recorder struct {
	mu      sync.Mutex
	state   State
	err     error
	path    string
	sources []Source
	// The time recorded is recorded plus, while recording, the span since
	// started: pauses add nothing to it.
	started  time.Time
	recorded time.Duration

	// enc encodes mixed audio to the file. samples carries delivered
	// buffers to the encode goroutine while recording (nil otherwise), and
	// done closes once that goroutine has finished and the file is complete.
	enc     encoder
	samples chan sourceSamples
	done    chan struct{}

	// notify asks the window for a frame from any goroutine, so the
	// elapsed time and the state show as they change.
	notify func()

	// platform is the OS-specific capture, nil where none is available.
	platform capture
	// newEncoder opens the output file; tests swap it for one that keeps the
	// samples in memory.
	newEncoder func(path string, bitrate int) (encoder, error)
}

// capture is the platform-specific audio capture behind the Recorder.
type capture interface {
	// available reports whether this platform can capture audio, and if
	// not, why.
	available() (bool, string)
	// start begins capturing the sources, delivering frames to deliver as
	// they arrive. It returns once capture is running or has failed.
	start(sources []Source, deliver func(Source, chunk)) error
	// stop ends capture; no more frames are delivered after it returns.
	stop()
}

func NewRecorder() *Recorder {
	return &Recorder{state: StateIdle, platform: newCapture(), newEncoder: newAACEncoder}
}

// available reports whether audio can be captured on this platform.
func (r *Recorder) Available() (bool, string) { return r.platform.available() }

// snapshot returns the Recorder's state for the UI, without holding the
// lock while the UI draws. elapsed is the time recorded, pauses left out,
// and holds the final length once the recording stops.
func (r *Recorder) Snapshot() (state State, elapsed time.Duration, path string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state, r.elapsedLocked(), r.path, r.err
}

// elapsedLocked is the time recorded so far. The caller holds mu.
func (r *Recorder) elapsedLocked() time.Duration {
	if r.state == StateRecording {
		return r.recorded + time.Since(r.started)
	}
	return r.recorded
}

// start captures the sources, encoding to path until stopped.
func (r *Recorder) Start(sources []Source, path string) {
	r.mu.Lock()
	if r.state != StateIdle {
		r.mu.Unlock()
		return
	}
	r.state = StateStarting
	r.err = nil
	r.path = path
	r.sources = sources
	r.recorded = 0
	r.mu.Unlock()
	r.ping()

	go func() {
		enc, err := r.newEncoder(path, aacBitRate)
		if err != nil {
			r.fail(err)
			return
		}
		sc := make(chan sourceSamples, 256)
		done := make(chan struct{})
		r.mu.Lock()
		r.enc, r.samples, r.done = enc, sc, done
		r.mu.Unlock()
		go r.encodeLoop(sources, enc, sc, done)

		if err := r.platform.start(sources, r.deliver); err != nil {
			// Capture never started: unwind the encoder and the empty file.
			r.mu.Lock()
			r.samples = nil
			r.mu.Unlock()
			close(sc)
			<-done
			enc.close()
			os.Remove(path)
			r.mu.Lock()
			r.enc, r.done = nil, nil
			r.state, r.err = StateIdle, err
			r.mu.Unlock()
			r.ping()
			return
		}

		r.mu.Lock()
		r.state, r.started = StateRecording, time.Now()
		r.mu.Unlock()
		r.ping()
		r.tick()
	}()
}

// fail records a startup error and returns to idle.
func (r *Recorder) fail(err error) {
	r.mu.Lock()
	r.state, r.err = StateIdle, err
	r.mu.Unlock()
	r.ping()
}

// deliver hands a source's buffer to the encode goroutine; called from the
// system's audio thread. It never blocks: if the encoder falls behind, the
// buffer is dropped rather than stall capture. While paused it drops every
// buffer, which is what keeps the pause out of the file.
func (r *Recorder) deliver(src Source, ch chunk) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.samples == nil || (r.state != StateRecording && r.state != StateStarting) {
		return
	}
	select {
	case r.samples <- sourceSamples{src: src, samples: ch.samples}:
	default:
	}
}

// encodeLoop mixes delivered buffers and encodes them until samples closes,
// then flushes the tail and signals done. It owns the mixer, so no lock is
// needed around it.
func (r *Recorder) encodeLoop(sources []Source, enc encoder, sc <-chan sourceSamples, done chan<- struct{}) {
	defer close(done)
	m := newMixer(sources)
	for s := range sc {
		if s.flush {
			if out := m.flush(); out != nil {
				r.encode(enc, out)
			}
			continue
		}
		m.add(s.src, s.samples)
		if out := m.drain(); out != nil {
			r.encode(enc, out)
		}
	}
	if out := m.flush(); out != nil {
		r.encode(enc, out)
	}
}

// encode writes one mixed span, keeping the first error for the UI.
func (r *Recorder) encode(enc encoder, out []float32) {
	if err := enc.write(out); err != nil {
		r.mu.Lock()
		if r.err == nil {
			r.err = err
		}
		r.mu.Unlock()
	}
}

// stop ends capture, finishes encoding and closes the file. A paused
// recording stops as it is.
func (r *Recorder) Stop() {
	r.mu.Lock()
	if r.state != StateRecording && r.state != StatePaused {
		r.mu.Unlock()
		return
	}
	r.recorded = r.elapsedLocked()
	r.state = StateStopping
	sc, done, enc := r.samples, r.done, r.enc
	r.samples = nil
	r.mu.Unlock()
	r.ping()

	go func() {
		r.platform.stop() // no more deliveries after this returns
		close(sc)         // end the encode loop
		<-done            // wait for the tail to be written
		cerr := enc.close()

		r.mu.Lock()
		if r.err == nil {
			r.err = cerr
		}
		r.enc, r.done, r.state = nil, nil, StateIdle
		r.mu.Unlock()
		r.ping()
	}()
}

// Pause stops taking in audio without ending the recording: capture keeps
// running and its buffers are dropped until Resume. The samples already
// handed to the encoder are mixed and written out first, so nothing from
// before the pause is held back to come out after it.
func (r *Recorder) Pause() {
	r.mu.Lock()
	if r.state != StateRecording {
		r.mu.Unlock()
		return
	}
	r.recorded = r.elapsedLocked()
	r.state = StatePaused
	if r.samples != nil {
		select {
		case r.samples <- sourceSamples{flush: true}:
		default:
		}
	}
	r.mu.Unlock()
	r.ping()
}

// Resume takes in audio again after Pause.
func (r *Recorder) Resume() {
	r.mu.Lock()
	if r.state != StatePaused {
		r.mu.Unlock()
		return
	}
	r.state, r.started = StateRecording, time.Now()
	r.mu.Unlock()
	r.ping()
}

// SetNotify sets the callback the recorder uses to ask the window for a
// frame from any goroutine, so the elapsed time and the state show as they
// change while recording.
func (r *Recorder) SetNotify(fn func()) {
	r.mu.Lock()
	r.notify = fn
	r.mu.Unlock()
}

// ping asks the window for a frame, if a window is listening.
func (r *Recorder) ping() {
	r.mu.Lock()
	notify := r.notify
	r.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// tick refreshes the elapsed time while recording. It keeps going through
// a pause, which a Resume ends, and stops with the recording.
func (r *Recorder) tick() {
	for range time.Tick(500 * time.Millisecond) {
		r.mu.Lock()
		st := r.state
		r.mu.Unlock()
		if st != StateRecording && st != StatePaused {
			return
		}
		if st == StateRecording {
			r.ping()
		}
	}
}

// mixer sums the enabled sources incrementally so audio can be encoded while
// recording. Each source's samples accumulate in a pending buffer; drain
// emits the span every source has delivered, leaving only the small arrival
// skew in memory.
type mixer struct {
	sources []Source
	pending map[Source][]float32
}

func newMixer(sources []Source) *mixer {
	return &mixer{sources: sources, pending: map[Source][]float32{}}
}

func (m *mixer) add(src Source, s []float32) {
	m.pending[src] = append(m.pending[src], s...)
}

// drain returns the longest prefix every source has delivered, summed and
// clipped, or nil when some source has nothing yet.
func (m *mixer) drain() []float32 {
	n := -1
	for _, src := range m.sources {
		if l := len(m.pending[src]); n < 0 || l < n {
			n = l
		}
	}
	if n <= 0 {
		return nil
	}
	return m.take(n)
}

// flush returns everything left, summed and clipped — the tail one source
// delivered beyond the others when recording stopped.
func (m *mixer) flush() []float32 {
	n := 0
	for _, src := range m.sources {
		if l := len(m.pending[src]); l > n {
			n = l
		}
	}
	if n == 0 {
		return nil
	}
	return m.take(n)
}

// take sums and removes the first n samples of every source's pending
// buffer, reusing each backing array so memory stays bounded. Samples that
// overflow the valid range are clipped.
func (m *mixer) take(n int) []float32 {
	out := make([]float32, n)
	for _, src := range m.sources {
		p := m.pending[src]
		k := n
		if len(p) < k {
			k = len(p)
		}
		for i := 0; i < k; i++ {
			out[i] += p[i]
		}
		m.pending[src] = p[:copy(p, p[k:])]
	}
	for i, v := range out {
		if v > 1 {
			out[i] = 1
		} else if v < -1 {
			out[i] = -1
		}
	}
	return out
}
