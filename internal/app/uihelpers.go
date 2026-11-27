package app

import (
	"fmt"
	"time"

	"github.com/Beeta/voxi/internal/store"
	"github.com/egoist/mygo/ui"
)

// Capture source display names, shared by the sidebar summary and the
// settings dialog.
const (
	labelMic    = "麦克风"
	labelSystem = "系统输出"
)

// controlFill is what a button fills with: rest while the pointer is away,
// hover while it is on, so a button answers before it is clicked.
//
// The framework does this itself through the theme, but only for a button
// whose fill it also set — it steps aside as soon as a view paints its own
// background, which every button here does.
func controlFill(e ui.Element, rest, hover ui.Color) ui.Color {
	if e.Hovered() {
		return hover
	}
	return rest
}

// actionButton is a tab's primary action in the player bar: an outlined
// accent button, dimmed to a muted outline while disabled. The bar already
// carries a filled accent play button, so these stay outlined rather than
// adding a second solid accent block. It returns the button so the caller
// reads Clicked; a disabled one never reports a click.
func actionButton(c *ui.Context, key, label string, off bool) ui.Element {
	fg, brd := accent, accent
	if off {
		fg, brd = muted, line
	}
	b := ui.ButtonBase(c.Key(key)).Padding(8, 16).Radius(8).Center().Border(1, brd).Disabled(off)
	b.Background(controlFill(b, cardBg2, cardBg3))
	b.Children(func() { ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(fg) })
	return b
}

func formatClock(d time.Duration) string {
	s := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

func formatSecs(sec float64) string {
	s := int(sec)
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

// recMeta is the one-line summary under a recording's name.
func recMeta(rec store.Recording) string {
	return recMetaStatus(rec, "")
}

// recMetaStatus is recMeta with status in place of 已识别 / 未识别 when it is
// set: the step a transcription under way is at.
func recMetaStatus(rec store.Recording, status string) string {
	if status == "" {
		status = "未识别"
		if rec.Transcript != nil {
			status = "已识别"
		}
	}
	meta := fmt.Sprintf("%s · %s · %s", rec.Created.Format("01-02 15:04"), formatSecs(rec.Duration), status)
	// The row says when there is no audio left to play: the recording is
	// still here for its text, and the player will not offer anything.
	if !hasAudio(rec) {
		meta += " · 无音频"
	}
	return meta
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// drawCheckbox draws a small check-box face for a hand-rolled toggle.
func drawCheckbox(c *ui.Context, on bool) {
	bg, brd := cardBg2, muted
	if on {
		bg, brd = accent, accent
	}
	box := ui.Column(c).Size(20, 20).Radius(6).Center().Background(bg).Border(1, brd)
	if on {
		box.Children(func() {
			ui.Icon(c, iconCheck).FontSize(13).TextColor(onAccent)
		})
	}
}

// icon parses a 24×24 stroked icon drawn in the current text color.
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconCheck       = icon(`<path d="M20 6 9 17l-5-5"/>`)
	iconPlay        = icon(`<path d="M6 4l14 8-14 8z" fill="currentColor" stroke="none"/>`)
	iconPause       = icon(`<path d="M7 4v16M17 4v16"/>`)
	iconStop        = icon(`<rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor" stroke="none"/>`)
	iconText        = icon(`<path d="M4 6h16M4 12h16M4 18h10"/>`)
	iconGear        = icon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>`)
	iconPlus        = icon(`<path d="M12 5v14M5 12h14"/>`)
	iconImport      = icon(`<path d="M12 3v12M8 11l4 4 4-4M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2"/>`)
	iconChevronDown = icon(`<path d="m6 9 6 6 6-6"/>`)
	iconTrash       = icon(`<path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6M10 11v6M14 11v6"/>`)
	// The marks a status carries, drawn rather than typed: ✓ and ✕ come out
	// of whatever font has them, at a weight and size of its own.
	iconChevronRight = icon(`<path d="m9 6 6 6-6 6"/>`)
	iconCross        = icon(`<path d="M18 6 6 18M6 6l12 12"/>`)
	iconCircle       = icon(`<circle cx="12" cy="12" r="7"/>`)
	iconAlert        = icon(`<path d="M12 3 2 20h20z"/><path d="M12 10v4M12 17h.01"/>`)
	// The settings pages' icons: 语音识别 and 大模型.
	iconMic     = icon(`<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3"/>`)
	iconSparkle = icon(`<path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8z"/><path d="M19 16v4M17 18h4"/>`)
)

// hoverFade is how a control's fill follows the pointer: quick enough to
// answer at once, long enough to not flicker as the pointer crosses a list.
var hoverFade = ui.Animation{Duration: 120 * time.Millisecond}

// pressPose makes b give a little while it is held, and spring back as it is
// let go, as a button in the system's apps does.
func pressPose(b ui.Element) {
	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = 0.96
	}
	b.AnimatePose(pose, ui.Snappy)
}

// secondaryButton is a quiet button beside a setting: a veil of fill that
// deepens under the pointer, and gives as it is pressed.
func secondaryButton(c *ui.Context, key, label string) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Padding(5, 12).Radius(7).Center().Border(1, line).Label(label)
	fill := hover
	if b.Hovered() {
		fill = press
	}
	b.Background(b.AnimateColor("bg", fill, hoverFade))
	pressPose(b)
	b.Children(func() { ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(ink).SingleLine() })
	return b
}

// segmented is a segmented control as AppKit draws one: segments of one
// width in a small sunken track, the one chosen raised on a capsule that
// slides to the next as the choice changes. It reports whether the user
// chose another, *selected already holding it.
func segmented(c *ui.Context, key string, selected *int, labels ...string) bool {
	const height, inset, radius = 26, 2, 7
	width := float32(0)
	for _, l := range labels {
		w, _ := c.MeasureText(0, ui.Span{Text: l, Size: 12.5, Weight: 500})
		width = max(width, w+24)
	}
	dark := c.Theme().Dark
	capsule := ui.Hex("#ffffff")
	if dark {
		capsule = ui.RGBA(255, 255, 255, 0.2)
	}
	before := *selected
	parts := ui.SegmentedBase(c.Key(key), selected, len(labels))
	at := parts.Track.AnimateTo("at", float32(*selected), ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.08}})
	parts.Track.Height(height).Padding(inset).Radius(radius).Background(press).Draw(func(p *ui.Painter, r ui.Rect) {
		pill := ui.Rect{X: r.X + inset + at*width, Y: r.Y + inset, W: width, H: r.H - 2*inset}
		if !dark {
			p.Shadow(pill, radius-1, 0, 0.5, 1, 0, ui.RGBA(0, 0, 0, 0.2))
			p.Shadow(pill, radius-1, 0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.06))
		}
		p.Fill(pill, capsule, radius-1)
	}).Children(func() {
		for i, l := range labels {
			seg := parts.Segment(i).Width(width).Height(height - 2*inset).Center().Radius(radius - 1).Label(l)
			seg.Children(func() { ui.Text(c, l).FontSize(12.5).FontWeight(500).TextColor(ink).SingleLine() })
		}
	})
	// The control writes a click through selected before it returns, so a
	// change shows as soon as it is built.
	return *selected != before
}

// disclosure is the chevron of a section that folds: pointing down while
// it is open, to the right while it is shut.
func disclosure(open bool) *ui.SVG {
	if open {
		return iconChevronDown
	}
	return iconChevronRight
}

// statusMark is an icon and a word in one color, the way a verified or a
// failed state reads at a glance.
func statusMark(c *ui.Context, svg *ui.SVG, text string, col ui.Color) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, svg).FontSize(12).TextColor(col)
		ui.Text(c, text).FontSize(12).TextColor(col)
	})
}

// warnLine is a warning: the alert mark, then text that wraps under
// itself rather than under the mark.
func warnLine(c *ui.Context, text string) ui.Element {
	var t ui.Element
	ui.Row(c).Gap(6).AlignItems(ui.Start).Children(func() {
		ui.Row(c).Height(17).Center().Shrink(0).Children(func() {
			ui.Icon(c, iconAlert).FontSize(12).TextColor(recRed)
		})
		t = ui.Text(c, text).FontSize(12).TextColor(recRed).Grow(1).MinWidth(0)
	})
	return t
}

func playIcon(playing bool) *ui.SVG {
	if playing {
		return iconPause
	}
	return iconPlay
}
