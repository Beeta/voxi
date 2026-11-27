package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beeta/voxi/internal/audio"
	"github.com/egoist/mygo/ui"
)

// The floater never says what it is: closed, it is a dot with no words; the
// pointer over it opens the pill with the clock, and the pill's buttons
// pause, resume and stop, clicked as the user would.
func TestFloaterOpensUnderThePointer(t *testing.T) {
	dir := t.TempDir()
	set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &Settings{Mic: true, RecordingsDir: dir}}
	a := newApp(openTestLib(t, dir), set, audio.NewSimulatedRecorder())
	// No window here: the hook only flips the state the real one would
	// after resizing.
	a.floatHover = func(open bool) { a.floatOpen.Store(open) }
	a.startRecording()
	for i := 0; i < 400; i++ {
		if st, _, _, _ := a.rec.Snapshot(); st == audio.StateRecording {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	tt := ui.NewTester(a.floatView, floatPillW, floatPillH)
	tt.Frame()
	if texts := tt.Texts(); len(texts) != 0 {
		t.Fatalf("closed, the floater shows %q; it must show no words", texts)
	}

	tt.Move(10, floatPillH/2)
	tt.Frame()
	if !a.floatOpen.Load() {
		t.Fatal("the pointer over the dot did not open the pill")
	}
	tt.Frame()
	if !tt.HasText("00:00") {
		t.Fatalf("open, the pill shows %q, want the clock", tt.Texts())
	}

	// The buttons are icons: the pause one is the first after the clock,
	// the stop one the last, 5 from the right edge.
	stopX := float32(floatPillW - 5 - 12)
	pauseX := stopX - 24 - 8
	tt.ClickAt(pauseX, floatPillH/2)
	tt.Frame()
	if st, _, _, _ := a.rec.Snapshot(); st != audio.StatePaused {
		t.Fatalf("the pause button left the recorder in state %d", st)
	}
	tt.ClickAt(pauseX, floatPillH/2)
	tt.Frame()
	if st, _, _, _ := a.rec.Snapshot(); st != audio.StateRecording {
		t.Fatalf("the resume button left the recorder in state %d", st)
	}
	tt.ClickAt(stopX, floatPillH/2)
	tt.Frame()
	for i := 0; i < 400; i++ {
		if st, _, _, _ := a.rec.Snapshot(); st == audio.StateIdle {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the stop button did not end the recording")
}

// The dot warms with the length of the recording: gray for the first half
// hour, amber by the hour, orange from two hours on, and never the red that
// would say it records.
func TestFloatTint(t *testing.T) {
	if got := floatTint(10 * time.Minute); got != floatGray {
		t.Errorf("at 10 min the dot is %v, want gray", got)
	}
	if got := floatTint(60 * time.Minute); got != floatAmber {
		t.Errorf("at 60 min the dot is %v, want amber", got)
	}
	if got := floatTint(3 * time.Hour); got != floatOrange {
		t.Errorf("at 3 h the dot is %v, want orange", got)
	}
	if mid := floatTint(45 * time.Minute); mid == floatGray || mid == floatAmber {
		t.Errorf("at 45 min the dot is %v, want between gray and amber", mid)
	}
}

// The floater's loop redraws only when what it shows changes: within a
// minute, closed and not quiet, the frame stays the same.
func TestFloatFrameChangesRarely(t *testing.T) {
	now := time.Unix(1000, 0)
	a := makeFloatFrame(audio.StateRecording, 61*time.Second, 0, false, now)
	b := makeFloatFrame(audio.StateRecording, 119*time.Second, 0, false, now.Add(58*time.Second))
	if a != b {
		t.Errorf("closed, the frame changed within a minute: %+v then %+v", a, b)
	}
	if c := makeFloatFrame(audio.StateRecording, 62*time.Second, 0, true, now); c == a {
		t.Error("open, the frame does not follow the clock")
	}
	q1 := makeFloatFrame(audio.StateRecording, time.Minute, quietRemind, false, now)
	q2 := makeFloatFrame(audio.StateRecording, time.Minute, quietRemind, false, now.Add(time.Second))
	if q1 == q2 {
		t.Error("quiet, the dot does not pulse")
	}
	if strings.Contains(remindText(5*time.Minute), "录") {
		t.Error("the quiet reminder says it records")
	}
}
