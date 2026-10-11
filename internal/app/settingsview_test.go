package app

import "testing"

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
