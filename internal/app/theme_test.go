package app

import (
	"math"
	"reflect"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// currentPalette reads the colors the views draw with as one palette, so a
// test can compare a whole appearance at once.
func currentPalette() palette {
	return palette{
		pageBg:    pageBg,
		cardBg:    cardBg,
		cardBg2:   cardBg2,
		cardBg3:   cardBg3,
		selBg:     selBg,
		line:      line,
		ink:       ink,
		muted:     muted,
		accent:    accent,
		onAccent:  onAccent,
		recRed:    recRed,
		okGreen:   okGreen,
		btnStroke: btnStroke,
		speakers:  speakers,
	}
}

// TestUseAppearanceDrawsTheWholePalette is useAppearance's contract: after
// it, every color the views read by name is the one that appearance carries.
// A color left out keeps the other appearance's value, which nothing else
// would catch — it draws fine, just wrong: a white label on a pale button.
func TestUseAppearanceDrawsTheWholePalette(t *testing.T) {
	defer useAppearance(ui.DarkTheme())

	for _, tc := range []struct {
		name  string
		theme *ui.Theme
		want  palette
	}{
		{"dark", ui.DarkTheme(), darkPalette},
		{"light", ui.LightTheme(), lightPalette},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useAppearance(tc.theme)
			if got := currentPalette(); got != tc.want {
				t.Errorf("the views draw with\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

// TestUseAppearanceColorsTheFrameworkTheme covers the other half: the
// widgets the app does not draw itself — text inputs, switches, selection,
// focus rings — read their colors from the theme, so it has to end up
// carrying the same palette the views draw with.
func TestUseAppearanceColorsTheFrameworkTheme(t *testing.T) {
	defer useAppearance(ui.DarkTheme())

	th := ui.LightTheme()
	useAppearance(th)
	for _, tc := range []struct {
		field string
		got   ui.Color
		want  ui.Color
	}{
		{"Background", th.Background, lightPalette.pageBg},
		{"Surface", th.Surface, lightPalette.cardBg},
		{"Border", th.Border, lightPalette.line},
		{"Text", th.Text, lightPalette.ink},
		{"TextMuted", th.TextMuted, lightPalette.muted},
		{"Accent", th.Accent, lightPalette.accent},
		{"AccentText", th.AccentText, lightPalette.onAccent},
		{"Danger", th.Danger, lightPalette.recRed},
		{"Success", th.Success, lightPalette.okGreen},
	} {
		if tc.got != tc.want {
			t.Errorf("the theme's %s is %v, want %v", tc.field, tc.got, tc.want)
		}
	}
}

// TestPalettesCarryEveryColor catches a color added to palette and filled in
// for one appearance only: the other would draw it as nothing. The speaker
// tints are a whole array per appearance, which IsZero only reports as unset
// when none of them is filled in, so they are checked one by one.
func TestPalettesCarryEveryColor(t *testing.T) {
	for _, tc := range []struct {
		name string
		pal  palette
	}{{"darkPalette", darkPalette}, {"lightPalette", lightPalette}} {
		v := reflect.ValueOf(tc.pal)
		for i := range v.NumField() {
			f := v.Field(i)
			if f.Kind() == reflect.Array {
				for j := range f.Len() {
					if f.Index(j).IsZero() {
						t.Errorf("%s.%s[%d] is unset", tc.name, v.Type().Field(i).Name, j)
					}
				}
				continue
			}
			if f.IsZero() {
				t.Errorf("%s.%s is unset", tc.name, v.Type().Field(i).Name)
			}
		}
	}
}

// TestSpeakerTintsDiffer keeps the tints telling speakers apart, which is the
// whole point of them: a repeated color, or two too close to tell apart,
// would make the transcript harder to read than one plain color.
func TestSpeakerTintsDiffer(t *testing.T) {
	for _, tc := range []struct {
		name string
		pal  palette
	}{{"darkPalette", darkPalette}, {"lightPalette", lightPalette}} {
		for i := range speakerTintCount {
			for j := i + 1; j < speakerTintCount; j++ {
				a, b := tc.pal.speakers[i], tc.pal.speakers[j]
				if a == b {
					t.Errorf("%s: speaker tints %d and %d are the same color", tc.name, i, j)
					continue
				}
				d := abs(int(a.R)-int(b.R)) + abs(int(a.G)-int(b.G)) + abs(int(a.B)-int(b.B))
				if d < 24 {
					t.Errorf("%s: speaker tints %d and %d differ by %d, too little to tell apart",
						tc.name, i, j, d)
				}
			}
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// relLum is a color's WCAG relative luminance.
func relLum(c ui.Color) float64 {
	ch := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
}

// contrastRatio is the WCAG contrast between two colors.
func contrastRatio(a, b ui.Color) float64 {
	la, lb := relLum(a)+0.05, relLum(b)+0.05
	if la < lb {
		la, lb = lb, la
	}
	return la / lb
}

// TestTextContrastMeetsAA pins secondary text (muted) at WCAG AA 4.5:1 over
// every surface it lands on, in both appearances. The light muted used to be
// the system's own ~3.3:1 secondary label; this keeps it from drifting back.
func TestTextContrastMeetsAA(t *testing.T) {
	for _, ap := range []struct {
		name string
		pal  palette
	}{{"dark", darkPalette}, {"light", lightPalette}} {
		for _, s := range []struct {
			where string
			bg    ui.Color
		}{
			{"page", ap.pal.pageBg}, {"card", ap.pal.cardBg},
			{"hovered row", ap.pal.cardBg2}, {"selected row", ap.pal.selBg},
		} {
			if got := contrastRatio(ap.pal.muted, s.bg); got < 4.5 {
				t.Errorf("%s: muted on %s is %.2f:1, below AA 4.5", ap.name, s.where, got)
			}
		}
	}
}

// TestAppearanceNamesRoundTrip pins the mapping the settings dialog leans
// on: the segmented control's index names the appearance that is saved and
// handed to the framework, and an unset one — an older settings file — is
// the system's.
func TestAppearanceNamesRoundTrip(t *testing.T) {
	for i, name := range appearanceNames {
		if got := appearanceIndex(name); got != i {
			t.Errorf("appearanceIndex(%q) = %d, want %d", name, got, i)
		}
	}
	if got := appearanceIndex(""); got != 0 {
		t.Errorf("appearanceIndex(\"\") = %d, want the system's 0", got)
	}
	if got := appearanceIndex("nonsense"); got != 0 {
		t.Errorf("appearanceIndex(\"nonsense\") = %d, want the system's 0", got)
	}
	for _, tc := range []struct {
		name string
		want mygo.ThemeSource
	}{
		{"system", mygo.ThemeSystem},
		{"light", mygo.ThemeLight},
		{"dark", mygo.ThemeDark},
		{"", mygo.ThemeSystem},
		{"nonsense", mygo.ThemeSystem},
	} {
		if got := appearanceSource(tc.name); got != tc.want {
			t.Errorf("appearanceSource(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestLightPaletteReversesTheSteps keeps the two appearances in the same
// relation to each other: a panel stands out from the page and a control on
// it recedes into the panel, whichever way round the background is. Read as
// luminance, the dark appearance climbs page → card → card2 and the light
// one climbs page → card and falls again to card2.
//
// cardBg3 is the same step again, the one a control takes under the pointer,
// so it has to continue in the direction cardBg2 was going — a hover that
// landed back on the resting color would answer nothing.
func TestLightPaletteReversesTheSteps(t *testing.T) {
	lum := func(c ui.Color) int { return int(c.R) + int(c.G) + int(c.B) }
	if lum(darkPalette.cardBg) <= lum(darkPalette.pageBg) {
		t.Error("in the dark appearance the panel is not lighter than the page")
	}
	if lum(darkPalette.cardBg2) <= lum(darkPalette.cardBg) {
		t.Error("in the dark appearance a control is not lighter than its panel")
	}
	if lum(darkPalette.cardBg3) <= lum(darkPalette.cardBg2) {
		t.Error("in the dark appearance a hovered control is not lighter than a resting one")
	}
	if lum(lightPalette.cardBg) <= lum(lightPalette.pageBg) {
		t.Error("in the light appearance the panel is not lighter than the page")
	}
	if lum(lightPalette.cardBg2) >= lum(lightPalette.cardBg) {
		t.Error("in the light appearance a control does not recede into its panel")
	}
	if lum(lightPalette.cardBg3) >= lum(lightPalette.cardBg2) {
		t.Error("in the light appearance a hovered control does not recede further than a resting one")
	}
}
