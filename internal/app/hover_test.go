package app

import (
	"image"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
)

// sidebarRowY is the vertical center of the sidebar's header row, where the
// title and the two icon buttons sit.
func sidebarRowY(tt *ui.Tester) float32 {
	title, ok := tt.Find("有听有记 Voxi")
	if !ok {
		return 0
	}
	return title.Y + title.H/2
}

// The sidebar is 280 wide with 20 of padding on each side, and its header row
// ends with the import button and the gear, each 28 wide (6 of padding around
// a 16 icon) with 8 between them. Their centers, measured from the left.
const (
	importX = 210
	gearX   = 246
)

// diffPixels is how much of the frame two renders disagree about, which is
// how a hovered button shows: its fill changes, and nothing else does.
func diffPixels(a, b *image.RGBA) int {
	n := 0
	for y := a.Rect.Min.Y; y < a.Rect.Max.Y; y++ {
		for x := a.Rect.Min.X; x < a.Rect.Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

// newHoverApp is an app whose sidebar shows one recording, over a settings
// file under t.TempDir().
func newHoverApp(t *testing.T, s Settings) *app {
	t.Helper()
	dir := t.TempDir()
	lib, err := store.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	lib.Add("录音", filepath.Join(dir, "a.m4a"), 60)
	set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &s}
	return newApp(lib, set, audio.NewRecorder())
}

// TestButtonsAnswerThePointer covers the sidebar's buttons: the pointer over
// one changes its fill, so a button says it can be clicked before it is. The
// framework would do this itself, but only for a button whose fill it also
// set — every button here paints its own, so the view has to answer.
func TestButtonsAnswerThePointer(t *testing.T) {
	a := newHoverApp(t, Settings{Mic: true, System: true})
	tt := ui.NewTester(a.view, 968, 704)
	tt.SetDark(true)
	tt.Frame()
	tt.Frame()

	y := sidebarRowY(tt)
	if y == 0 {
		t.Skip("the sidebar did not render")
	}
	rest := tt.Image()

	for _, tc := range []struct {
		name string
		x    float32
		want bool
	}{
		{"the settings gear", gearX, true},
		{"the import button", importX, true},
		// The title between them is not a button: nothing may change.
		{"the title, which is not a button", 140, false},
	} {
		tt.Move(tc.x, y)
		tt.Frame()
		tt.Frame()
		if got := diffPixels(rest, tt.Image()) > 0; got != tc.want {
			t.Errorf("the pointer on %s changed the frame: %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestDisabledButtonStaysPut is the other half: the record button cannot be
// clicked with no source chosen, and it must not answer the pointer as if it
// could.
func TestDisabledButtonStaysPut(t *testing.T) {
	for _, tc := range []struct {
		name  string
		s     Settings
		hover bool
	}{
		{"with a source chosen", Settings{Mic: true}, true},
		{"with no source chosen", Settings{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newHoverApp(t, tc.s)
			tt := ui.NewTester(a.view, 968, 704)
			tt.SetDark(true)
			tt.Frame()
			tt.Frame()
			rec, ok := tt.Find("开始录制")
			if !ok {
				t.Skip("the sidebar did not render")
			}
			rest := tt.Image()
			tt.Move(rec.X+rec.W/2, rec.Y+rec.H/2)
			tt.Frame()
			tt.Frame()
			if got := diffPixels(rest, tt.Image()) > 0; got != tc.hover {
				t.Errorf("the pointer on the record button changed the frame: %v, want %v", got, tc.hover)
			}
		})
	}
}

// TestIconButtonsShowTheirTip covers what the two icon buttons have instead of
// a label: a tip that says what they do, once the pointer has rested on them.
// Before that it must stay away — a tip that appears as the pointer crosses a
// button is noise, not help.
//
// Each button is tried on a window of its own: once a tip has been shown, the
// framework warms up and shows the next one at once, as a toolbar's tips do.
func TestIconButtonsShowTheirTip(t *testing.T) {
	for _, tc := range []struct {
		name string
		x    float32
		tip  string
	}{
		{"the settings gear", gearX, "设置"},
		{"the import button", importX, "导入音频文件"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newHoverApp(t, Settings{Mic: true, System: true})
			tt := ui.NewTester(a.view, 968, 704)
			tt.SetDark(true)
			tt.Frame()
			tt.Frame()

			y := sidebarRowY(tt)
			if y == 0 {
				t.Skip("the sidebar did not render")
			}
			tt.Move(tc.x, y)
			tt.Frame()
			if tt.HasText(tc.tip) {
				t.Errorf("the tip appeared at once, before the pointer rested")
			}
			// The framework waits out tooltipDelay (600ms) before showing it.
			time.Sleep(750 * time.Millisecond)
			tt.Frame()
			tt.Frame()
			if !tt.HasText(tc.tip) {
				t.Errorf("resting on the button showed no tip, want %q", tc.tip)
			}
			// Off the button, and the tip goes with it.
			tt.Move(140, y)
			tt.Frame()
			tt.Frame()
			if tt.HasText(tc.tip) {
				t.Errorf("the tip stayed after the pointer left")
			}
		})
	}
}
