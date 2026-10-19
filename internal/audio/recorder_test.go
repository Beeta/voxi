package audio

import (
	"path/filepath"
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
	r, _, _ := fakeRecorder(t)
	r.Start([]Source{SourceMic}, filepath.Join(t.TempDir(), "x.m4a"))
	waitState(t, r, StateRecording)
	time.Sleep(60 * time.Millisecond)
	r.Pause()
	_, atPause, _, _ := r.Snapshot()
	time.Sleep(120 * time.Millisecond)
	_, later, _, _ := r.Snapshot()
	if later != atPause {
		t.Errorf("the time went on while paused: %v then %v", atPause, later)
	}
	if atPause < 50*time.Millisecond || atPause > 110*time.Millisecond {
		t.Errorf("recorded %v before the pause, want about 60ms", atPause)
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
