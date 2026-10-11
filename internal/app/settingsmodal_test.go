package app

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/zhaorongcun/voxi/internal/audio"
	"github.com/zhaorongcun/voxi/internal/store"
)

// TestSettingsDialogFitsTheWindow renders the real settings dialog at the
// smallest window the app allows and checks that its body ends inside the
// window. The body used to be a fixed height, and the scroll does not shrink
// to fit on its own: the dialog grew past the window and the settings below
// the fold became unreachable, since the bottom of the body was cut off.
//
// An overflowing body reports the window's bottom edge as its own, so the
// check is that it ends strictly above it.
func TestSettingsDialogFitsTheWindow(t *testing.T) {
	for _, h := range []int{480, 704} {
		dir := t.TempDir()
		lib, err := store.OpenStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &Settings{}}
		a := newApp(lib, set, audio.NewRecorder())
		a.showSettings = true

		const width = 720
		tt := ui.NewTester(a.view, width, h)
		// The first frame has no measured window to size the body against;
		// the second is the one the user sees.
		tt.Frame()
		tt.Frame()

		body, ok := tt.Find("设置内容")
		if !ok {
			// Capture is unavailable off macOS, where the view renders a
			// notice instead of the dialog.
			t.Skip("the settings dialog did not render")
		}
		bottom := body.Y + body.H
		if bottom >= float32(h) {
			t.Errorf("window %d high: the settings body ends at %v, so the dialog overflows it",
				h, bottom)
		}
		if want := settingsBodyHeight(float32(h)); body.H != want {
			t.Errorf("window %d high: the body is %v tall, want %v — it was squeezed",
				h, body.H, want)
		}
	}
}
