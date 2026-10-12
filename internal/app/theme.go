package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// speakerTintCount is how many speakers get a color of their own before the
// transcript starts reusing them.
const speakerTintCount = 6

// palette is one appearance's colors. The app draws with its own values
// rather than the framework theme's, which is what makes a light appearance
// a second palette and nothing else.
type palette struct {
	pageBg   ui.Color // the window behind everything
	cardBg   ui.Color // panels: the sidebar, cards, bubbles
	cardBg2  ui.Color // controls on a panel, and the selected row
	cardBg3  ui.Color // a control on a panel under the pointer
	line     ui.Color // separators and outlines
	ink      ui.Color // primary text
	muted    ui.Color // secondary text, and what is disabled
	accent   ui.Color // the primary action, and the selection
	onAccent ui.Color // text and icons on accent
	recRed   ui.Color // recording, and what needs attention
	okGreen  ui.Color // transcribed, verified
	// btnStroke outlines a cardBg2 button, which is too close to its own
	// fill to read as a button without it.
	btnStroke ui.Color
	// speakers are the fills of the transcript's speaker chips: the first
	// speaker of a recording takes the first, the next a different one, so
	// who said what is readable at a glance. They are tints rather than
	// saturated colors — a transcript is mostly text, and the chips should
	// sort it, not shout over it. ink sits on all of them.
	speakers [speakerTintCount]ui.Color
}

// darkPalette is macOS's dark system colors, which the app has drawn with
// since it had only one appearance.
var darkPalette = palette{
	pageBg:    ui.Hex("#1c1c1e"),
	cardBg:    ui.Hex("#2c2c2e"),
	cardBg2:   ui.Hex("#333335"),
	cardBg3:   ui.Hex("#3f3f43"),
	line:      ui.Hex("#3a3a3c"),
	ink:       ui.Hex("#f5f5f7"),
	muted:     ui.Hex("#9a9a9e"),
	accent:    ui.Hex("#0a84ff"),
	onAccent:  ui.Hex("#ffffff"),
	recRed:    ui.Hex("#ff453a"),
	okGreen:   ui.Hex("#30d158"),
	btnStroke: ui.Hex("#ffffff33"),
	// Deep tints of the same hues, dark enough for near-white ink to sit on
	// and still read as different colors from one another.
	speakers: [speakerTintCount]ui.Color{
		ui.Hex("#1f3a5c"), // blue
		ui.Hex("#362a55"), // purple
		ui.Hex("#4d2740"), // pink
		ui.Hex("#4a3520"), // orange
		ui.Hex("#1b3f3d"), // teal
		ui.Hex("#24402c"), // green
	},
}

// lightPalette is the same palette turned over: the same macOS colors in
// their light appearance, so a panel is still the thing that stands out
// from the page and cardBg2 still the thing that recedes into it — the
// direction of each step reverses with the background.
var lightPalette = palette{
	pageBg:    ui.Hex("#f2f2f7"),
	cardBg:    ui.Hex("#ffffff"),
	cardBg2:   ui.Hex("#e5e5ea"),
	cardBg3:   ui.Hex("#d6d6dc"),
	line:      ui.Hex("#d1d1d6"),
	ink:       ui.Hex("#1c1c1e"),
	muted:     ui.Hex("#8e8e93"),
	accent:    ui.Hex("#007aff"),
	onAccent:  ui.Hex("#ffffff"),
	recRed:    ui.Hex("#ff3b30"),
	okGreen:   ui.Hex("#34c759"),
	btnStroke: ui.Hex("#0000001a"),
	// Pale tints of the same hues, dark enough under the light appearance's
	// near-black ink and light enough to stand off a white panel.
	speakers: [speakerTintCount]ui.Color{
		ui.Hex("#d7e6fb"), // blue
		ui.Hex("#e4dcf8"), // purple
		ui.Hex("#fbdee9"), // pink
		ui.Hex("#fbe6d0"), // orange
		ui.Hex("#d3efeb"), // teal
		ui.Hex("#dcefdb"), // green
	},
}

// The colors the views draw with, by name. view() refills them from the
// current appearance before anything reads them, so a view never has to
// carry the appearance around.
//
// Package state is safe here: view() runs on the UI thread and every read
// happens inside its call. They start out dark so a view drawn on its own —
// a test — still gets sensible colors.
var (
	pageBg    = darkPalette.pageBg
	cardBg    = darkPalette.cardBg
	cardBg2   = darkPalette.cardBg2
	cardBg3   = darkPalette.cardBg3
	line      = darkPalette.line
	ink       = darkPalette.ink
	muted     = darkPalette.muted
	accent    = darkPalette.accent
	onAccent  = darkPalette.onAccent
	recRed    = darkPalette.recRed
	okGreen   = darkPalette.okGreen
	btnStroke = darkPalette.btnStroke
	speakers  = darkPalette.speakers
)

// useAppearance takes t's appearance as the one to draw with: it refills
// the palette above and writes the same colors into t, so the framework's
// own widgets — selection, focus rings, scrollbars, text inputs — follow
// along instead of keeping the framework's colors under the app's.
//
// Everything t does not carry is left as it is: Selection, Focus and
// Scrollbar come from the framework theme for this appearance, and are
// right already.
func useAppearance(t *ui.Theme) {
	p := lightPalette
	if t.Dark {
		p = darkPalette
	}
	pageBg, cardBg, cardBg2, cardBg3, line = p.pageBg, p.cardBg, p.cardBg2, p.cardBg3, p.line
	ink, muted, accent, onAccent = p.ink, p.muted, p.accent, p.onAccent
	recRed, okGreen, btnStroke = p.recRed, p.okGreen, p.btnStroke
	speakers = p.speakers

	t.Background = p.pageBg
	t.Surface = p.cardBg
	t.SurfaceHover, t.SurfacePressed = p.cardBg2, p.cardBg2
	t.Border = p.line
	t.Text, t.TextMuted = p.ink, p.muted
	t.Accent, t.AccentText = p.accent, p.onAccent
	t.Danger, t.Success = p.recRed, p.okGreen
}

// The appearance choices, in the order the settings dialog shows them.
// The names are what settings.json holds; "" is the system's appearance,
// which is also what an older settings file means.
var (
	appearanceNames  = []string{"system", "light", "dark"}
	appearanceLabels = []string{"跟随系统", "浅色", "深色"}
)

// appearanceIndex is where name sits in the segmented control.
func appearanceIndex(name string) int {
	for i, n := range appearanceNames {
		if n == name {
			return i
		}
	}
	return 0
}

// appearanceSource is the framework's name for the appearance name.
func appearanceSource(name string) mygo.ThemeSource {
	switch name {
	case "light":
		return mygo.ThemeLight
	case "dark":
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}
