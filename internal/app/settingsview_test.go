package app

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
)

// newSettingsApp is an app over a settings file under t.TempDir(), so a test
// never reads or writes the user's own.
func newSettingsApp(t *testing.T) (*app, *settingsStore) {
	t.Helper()
	dir := t.TempDir()
	lib := openTestLib(t, dir)
	set := &settingsStore{
		path: filepath.Join(dir, "settings.json"),
		s:    &Settings{Mic: true, System: true},
	}
	return newApp(lib, set, audio.NewRecorder()), set
}

// TestAppearanceControlSavesTheChoice clicks 浅色 in the settings dialog and
// wants the choice saved and handed to the framework.
//
// A segmented control writes the chosen index into the bound value at the
// end of the pass it was clicked in, so the section has to ask Changed: a
// copy of the saved index compared right after the call is still the old one
// and the click goes nowhere. That is why this test drives the real control
// through a real click — asking appearanceIndex alone would not catch it.
func TestAppearanceControlSavesTheChoice(t *testing.T) {
	a, set := newSettingsApp(t)
	var handed []mygo.ThemeSource
	a.setTheme = func(s mygo.ThemeSource) { handed = append(handed, s) }

	tt := ui.NewTester(func(c *ui.Context) {
		a.appearanceSettings(c, set.snapshot())
	}, 480, 200)

	if err := tt.Click("浅色"); err != nil {
		t.Fatal(err)
	}
	if got := set.snapshot().Appearance; got != "light" {
		t.Errorf("clicking 浅色 saved appearance %q, want %q", got, "light")
	}
	if len(handed) != 1 || handed[0] != mygo.ThemeLight {
		t.Errorf("the framework was handed %v, want one %q", handed, mygo.ThemeLight)
	}
}

// TestSwitchRowSavesTheToggle clicks the microphone switch in the settings
// dialog and wants the toggle saved. Same trap as the appearance control: a
// switch applies its click at the end of the pass, so the new value is only
// there to read through Changed.
func TestSwitchRowSavesTheToggle(t *testing.T) {
	const width = 480
	a, set := newSettingsApp(t)
	tt := ui.NewTester(func(c *ui.Context) {
		a.recordSettings(c, set.snapshot())
	}, width, 200)

	// The switch is the right end of the row: the description names the row,
	// and the switch sits past the text column that fills it.
	row, ok := tt.Find("默认输入设备（麦克风、耳机麦）")
	if !ok {
		t.Fatal("the microphone row did not render")
	}
	tt.ClickAt(width-10, row.Y+row.H/2)
	if set.snapshot().Mic {
		t.Error("clicking the microphone switch left it on")
	}
}

// TestSettingsBodyHeight pins the body height the settings dialog asks for:
// it shrinks with the window so the dialog fits, falls back to the value it
// used to hardcode before the window is measured, and never goes so small
// that the body is unusable.
func TestSettingsBodyHeight(t *testing.T) {
	cases := []struct {
		name    string
		windowH float32
		want    float32
	}{
		{"unmeasured window", 0, settingsBodyFallback},
		{"default window 968x704", 704, 704 - settingsBodyChrome},
		{"smallest window 720x480", 480, 480 - settingsBodyChrome},
		{"shorter than the chrome allows", 300, settingsBodyFloor},
	}
	for _, tc := range cases {
		if got := settingsBodyHeight(tc.windowH); got != tc.want {
			t.Errorf("%s: settingsBodyHeight(%v) = %v, want %v",
				tc.name, tc.windowH, got, tc.want)
		}
	}
}

// TestSettingsBodyFitsTheWindow is the property that matters: wherever the
// body is not already at its floor, the dialog leaves room for the chrome
// around the body inside the window.
func TestSettingsBodyFitsTheWindow(t *testing.T) {
	for h := float32(400); h <= 1400; h += 4 {
		body := settingsBodyHeight(h)
		if body == settingsBodyFloor {
			continue // the floor: the window is too short to fit it all
		}
		if body+settingsBodyChrome > h {
			t.Fatalf("window %v: body %v plus chrome overflows the window", h, body)
		}
	}
}

// TestSettingsPages: the dialog shows one page at a time, switched by the
// tabs over it, and the Q&A presets are not among them (they live on the
// Q&A tab).
func TestSettingsPages(t *testing.T) {
	a, _ := newSettingsApp(t)
	a.showSettings = true
	tt := ui.NewTester(func(c *ui.Context) { a.settingsModal(c, 704) }, 968, 704)
	tt.Frame()
	if !tt.HasText("主题") || tt.HasText("大模型配置") {
		t.Fatalf("the first page shows %q", tt.Texts())
	}
	if err := tt.Click("大模型"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("大模型配置") || tt.HasText("主题") {
		t.Errorf("after 大模型 the dialog shows %q", tt.Texts())
	}
	if tt.HasText("问答预设") {
		t.Error("the presets are still in settings")
	}
}
