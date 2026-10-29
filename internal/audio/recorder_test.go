package audio

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeCapture stands in for ScreenCaptureKit: push delivers a buffer as the
// system's audio thread would.
type fakeCapture struct {
	mu      sync.Mutex
	deliver func(Source, chunk)
	stopped bool
}

func (f *fakeCapture) available() (bool, string) { return true, "" }
func (f *fakeCapture) start(_ []Source, d func(Source, chunk)) error {
	f.mu.Lock()
	f.deliver = d
	f.mu.Unlock()
	return nil
}
func (f *fakeCapture) stop() {
	f.mu.Lock()
	f.stopped = true
	f.mu.Unlock()
}
func (f *fakeCapture) push(src Source, v float32, n int) {
	s := make([]float32, n)
	for i := range s {
		s[i] = v
	}
	f.mu.Lock()
	d := f.deliver
	f.mu.Unlock()
	d(src, chunk{samples: s})
}

// memEncoder keeps what it is given.
type memEncoder struct {
	mu  sync.Mutex
	out []float32
}

func (e *memEncoder) write(s []float32) error {
	e.mu.Lock()
	e.out = append(e.out, s...)
	e.mu.Unlock()
	return nil
}
func (e *memEncoder) close() error { return nil }
func (e *memEncoder) samples() []float32 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]float32(nil), e.out...)
}

func fakeRecorder(t *testing.T) (*Recorder, *fakeCapture, *memEncoder) {
	t.Helper()
	fc, enc := &fakeCapture{}, &memEncoder{}
	r := NewSimulatedRecorder()
	r.platform = fc
	r.newEncoder = func(string, int) (encoder, error) { return enc, nil }
	return r, fc, enc
}

func waitState(t *testing.T, r *Recorder, want State) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if st, _, _, _ := r.Snapshot(); st == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	st, _, _, _ := r.Snapshot()
	t.Fatalf("state %d, want %d", st, want)
}

// waitLen waits for the encoder to have taken n samples.
func waitLen(t *testing.T, enc *memEncoder, n int) {
	t.Helper()
	for i := 0; i < 400 && len(enc.samples()) < n; i++ {
		time.Sleep(5 * time.Millisecond)
	}
}

// TestPauseLeavesTheGapOut: what is delivered while paused never reaches the
// file, and what comes after resuming follows straight on.
func TestPauseLeavesTheGapOut(t *testing.T) {
	r, fc, enc := fakeRecorder(t)
	r.Start([]Source{SourceMic}, filepath.Join(t.TempDir(), "x.m4a"))
	waitState(t, r, StateRecording)

	fc.push(SourceMic, 0.1, 4)
	waitLen(t, enc, 4)
	r.Pause()
	if st, _, _, _ := r.Snapshot(); st != StatePaused {
		t.Fatalf("after Pause the state is %d", st)
	}
	fc.push(SourceMic, 0.9, 6) // spoken during the pause
	r.Resume()
	fc.push(SourceMic, 0.2, 4)
	r.Stop()
	waitState(t, r, StateIdle)

	want := []float32{0.1, 0.1, 0.1, 0.1, 0.2, 0.2, 0.2, 0.2}
	got := enc.samples()
	if len(got) != len(want) {
		t.Fatalf("the file holds %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the file holds %v, want %v", got, want)
		}
	}
}

// TestPauseFlushesTheSkew: with two sources, the part one delivered ahead of
// the other is written out at the pause rather than coming out after it.
func TestPauseFlushesTheSkew(t *testing.T) {
	r, fc, enc := fakeRecorder(t)
	r.Start([]Source{SourceMic, SourceSystem}, filepath.Join(t.TempDir(), "x.m4a"))
	waitState(t, r, StateRecording)
	fc.push(SourceMic, 0.1, 4)
	fc.push(SourceSystem, 0.1, 2) // 2 behind the mic
	waitLen(t, enc, 2)
	r.Pause()
	waitLen(t, enc, 4)
	if n := len(enc.samples()); n != 4 {
		t.Fatalf("after the pause %d samples are written, want the 4 delivered", n)
	}
	r.Resume()
	r.Stop()
	waitState(t, r, StateIdle)
}

// TestPausedTimeIsNotCounted: the elapsed time stands still while paused,
// and a paused recording stops with the time recorded until the pause.
func TestPausedTimeIsNotCounted(t *testing.T) {
	r, fc, enc := fakeRecorder(t)
	r.Start([]Source{SourceMic}, filepath.Join(t.TempDir(), "x.m4a"))
	waitState(t, r, StateRecording)
	// Whole seconds: the mixer emits in spans as sources arrive, and a
	// remainder below one span waits for the next buffer.
	fc.push(SourceMic, 0, sampleRate*channels)
	waitLen(t, enc, sampleRate*channels)
	waitUntil(t, func() bool { _, el, _, _ := r.Snapshot(); return el > 50*time.Millisecond })
	r.Pause()
	_, atPause, _, _ := r.Snapshot()
	if atPause < 900*time.Millisecond || atPause > 1100*time.Millisecond {
		t.Errorf("recorded %v before the pause, want about a second of audio", atPause)
	}
	fc.push(SourceMic, 0, sampleRate*channels) // delivered while paused: not written
	time.Sleep(120 * time.Millisecond)
	_, later, _, _ := r.Snapshot()
	if later != atPause {
		t.Errorf("the time went on while paused: %v then %v", atPause, later)
	}
	r.Stop()
	waitState(t, r, StateIdle)
	if _, final, _, _ := r.Snapshot(); final != atPause {
		t.Errorf("stopping while paused kept %v, want %v", final, atPause)
	}
}

// TestPauseResumeOnlyWhereTheyApply: Pause and Resume do nothing outside the
// state they belong to.
func TestPauseResumeOnlyWhereTheyApply(t *testing.T) {
	r, _, _ := fakeRecorder(t)
	r.Pause()
	r.Resume()
	if st, _, _, _ := r.Snapshot(); st != StateIdle {
		t.Errorf("Pause/Resume on an idle recorder left it in %d", st)
	}
	r.Start([]Source{SourceMic}, filepath.Join(t.TempDir(), "x.m4a"))
	waitState(t, r, StateRecording)
	r.Resume()
	if st, _, _, _ := r.Snapshot(); st != StateRecording {
		t.Errorf("Resume while recording changed the state to %d", st)
	}
	r.Stop()
	waitState(t, r, StateIdle)
}

// TestTimeFollowsTheAudio: the elapsed time is what has been written, not
// wall time — an empty recording reports nothing however long it ran.
func TestTimeFollowsTheAudio(t *testing.T) {
	r, fc, enc := fakeRecorder(t)
	r.Start([]Source{SourceMic}, filepath.Join(t.TempDir(), "x.m4a"))
	waitState(t, r, StateRecording)
	time.Sleep(120 * time.Millisecond)
	if _, el, _, _ := r.Snapshot(); el > 10*time.Millisecond {
		t.Errorf("with no audio, elapsed is %v", el)
	}
	fc.push(SourceMic, 0, sampleRate*channels) // one second
	waitLen(t, enc, sampleRate*channels)
	waitUntil(t, func() bool { _, el, _, _ := r.Snapshot(); return el >= 900*time.Millisecond })
	if _, el, _, _ := r.Snapshot(); el > 1100*time.Millisecond {
		t.Errorf("after a second of audio, elapsed is %v", el)
	}
	r.Stop()
	waitState(t, r, StateIdle)
}

// stalledCapture stops delivering after it is first started over.
type stalledCapture struct {
	fakeCapture
	starts int
}

func (s *stalledCapture) start(srcs []Source, d func(Source, chunk)) error {
	s.starts++
	if s.starts > 1 {
		return nil // the capture "restarts" but never delivers again
	}
	return s.fakeCapture.start(srcs, d)
}

// TestStalledCaptureIsRestarted: when the capture stops delivering — as the
// system's disk-space purge does without telling anyone — the recorder
// notices, starts it again, and writes the missing time as silence, so the
// file keeps real time and the timer does not run on past the audio.
func TestStalledCaptureIsRestarted(t *testing.T) {
	fc, enc := &stalledCapture{}, &memEncoder{}
	r := NewSimulatedRecorder()
	r.platform = fc
	r.newEncoder = func(string, int) (encoder, error) { return enc, nil }
	r.stallAfter = 100 * time.Millisecond
	r.Start([]Source{SourceMic}, filepath.Join(t.TempDir(), "x.m4a"))
	waitState(t, r, StateRecording)
	fc.push(SourceMic, 0, sampleRate*channels)
	waitLen(t, enc, sampleRate*channels)

	waitUntil(t, func() bool { return r.Silence() > 0 })
	if fc.starts < 2 {
		t.Fatalf("the capture was started %d times, want it restarted", fc.starts)
	}
	waitUntil(t, func() bool { return len(enc.samples()) > sampleRate*channels })
	r.Stop()
	waitState(t, r, StateIdle)
	if got := len(enc.samples()) / channels; float64(got) < 0.9*sampleRate {
		t.Errorf("the file holds %d frames, want the second of audio plus the silence", got)
	}
}

// TestRecoveryFailureIsReported: a capture that cannot be restarted leaves
// an error for the UI rather than a recording that goes on empty.
func TestRecoveryFailureIsReported(t *testing.T) {
	fc := &failingRestart{fakeCapture: fakeCapture{}}
	r := NewSimulatedRecorder()
	r.platform = fc
	r.newEncoder = func(string, int) (encoder, error) { return &memEncoder{}, nil }
	r.stallAfter = 100 * time.Millisecond
	r.Start([]Source{SourceMic}, filepath.Join(t.TempDir(), "x.m4a"))
	waitState(t, r, StateRecording)
	waitUntil(t, func() bool { _, _, _, err := r.Snapshot(); return err != nil })
	if _, _, _, err := r.Snapshot(); !strings.Contains(err.Error(), "自动恢复失败") {
		t.Errorf("the error is %v", err)
	}
	r.Stop()
	waitState(t, r, StateIdle)
}

type failingRestart struct {
	fakeCapture
	starts int
}

func (f *failingRestart) start(srcs []Source, d func(Source, chunk)) error {
	f.starts++
	if f.starts > 1 {
		return errors.New("权限被撤销")
	}
	return f.fakeCapture.start(srcs, d)
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out")
}
