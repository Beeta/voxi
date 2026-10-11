package app

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/zhaorongcun/voxi/internal/audio"
	"github.com/zhaorongcun/voxi/internal/store"
)

// TestSettingsDialogFitsTheWindow renders the real settings dialog at a
// short window, the default one, and a tall one, and checks that its body
// stays inside the window. The body used to be a fixed height, and the
// scroll does not shrink to fit on its own: in a short window the dialog
// grew past it and the settings below the fold became unreachable, since the
// bottom of the body was cut off.
//
// An overflowing body reports the window's bottom edge as its own, so the
// check is that it ends strictly above it.
func TestSettingsDialogFitsTheWindow(t *testing.T) {
	for _, h := range []int{480, 704, 900} {
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
		if !tt.HasText("版本") {
			t.Error("the dialog does not say which version it is")
		}
		if bottom := body.Y + body.H; bottom >= float32(h) {
			t.Errorf("window %d high: the settings body ends at %v, so the dialog overflows it",
				h, bottom)
		}
		// The cap is a maximum: a tall window gets a body only as tall as
		// the settings in it.
		if asked := settingsBodyHeight(float32(h)); body.H > asked {
			t.Errorf("window %d high: the body is %v tall, more than the %v asked for",
				h, body.H, asked)
		}
	}
}
