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

	tt := ui.NewTester(a.floatView, floatPillW, floatWin)
	tt.Frame()
	if texts := tt.Texts(); len(texts) != 1 || texts[0] != "⏰" {
		t.Fatalf("closed, the floater shows %q; it must show the alarm clock and no words", texts)
	}

	tt.Move(floatWin/2, floatWin/2)
	tt.Frame()
	if !a.floatOpen.Load() {
		t.Fatal("the pointer over the dot did not open the pill")
	}
	tt.Frame()
	if !tt.HasText("00:00") {
		t.Fatalf("open, the pill shows %q, want the clock", tt.Texts())
	}

	// The buttons are icons: the pause one is the first after the clock,
	// the stop one the last, inside the box's margin and padding.
	stopX := float32(floatPillW - floatInset - 5 - 12)
	pauseX := stopX - 24 - 6
	tt.ClickAt(pauseX, floatWin/2)
	tt.Frame()
	if st, _, _, _ := a.rec.Snapshot(); st != audio.StatePaused {
		t.Fatalf("the pause button left the recorder in state %d", st)
	}
	tt.ClickAt(pauseX, floatWin/2)
	tt.Frame()
	if st, _, _, _ := a.rec.Snapshot(); st != audio.StateRecording {
		t.Fatalf("the resume button left the recorder in state %d", st)
	}
	tt.ClickAt(stopX, floatWin/2)
	tt.Frame()
	for i := 0; i < 400; i++ {
		if st, _, _, _ := a.rec.Snapshot(); st == audio.StateIdle {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the stop button did not end the recording")
}

// The ring goes green, yellow, orange, red as the recording gets longer,
// blending between the stops.
func TestFloatTint(t *testing.T) {
	for _, tc := range []struct {
		at   time.Duration
		want ui.Color
		name string
	}{
		{10 * time.Minute, floatGreen, "green"},
		{30 * time.Minute, floatYellow, "yellow"},
		{45 * time.Minute, floatOrange, "orange"},
		{2 * time.Hour, floatRed, "red"},
	} {
		if got := floatTint(tc.at); got != tc.want {
			t.Errorf("at %v the ring is %v, want %s", tc.at, got, tc.name)
		}
	}
	if mid := floatTint(22 * time.Minute); mid == floatGreen || mid == floatYellow {
		t.Errorf("at 22 min the ring is %v, want between green and yellow", mid)
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

// The closed floater bounces once every floatBounceEvery of recording, and
// not as the recording starts.
func TestFloatBounceEveryTenMinutes(t *testing.T) {
	now := time.Unix(1000, 0)
	at := func(d time.Duration) int {
		return makeFloatFrame(audio.StateRecording, d, 0, false, now).bounce
	}
	if at(0) != 0 || at(9*time.Minute+59*time.Second) != 0 {
		t.Error("the floater bounces before ten minutes")
	}
	if at(10*time.Minute) != 1 || at(25*time.Minute) != 2 {
		t.Errorf("bounce count at 10 and 25 min is %d and %d, want 1 and 2", at(10*time.Minute), at(25*time.Minute))
	}
	if fs := bounceFrames(0); fs[0].Value != 1 || fs[len(fs)-1].Value != 1 {
		t.Error("a recording just started bounces")
	}
	for _, k := range bounceFrames(1) {
		if k.Value*floatBox > floatWin {
			t.Errorf("the bounce grows the circle to %v, past the %d window", k.Value*floatBox, floatWin)
		}
	}
}
