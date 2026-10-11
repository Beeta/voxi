package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/zhaorongcun/voxi/internal/store"
)

// Capture source display names, shared by the sidebar summary and the
// settings dialog.
const (
	labelMic    = "麦克风"
	labelSystem = "系统输出"
)

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
	status := "未识别"
	if rec.Transcript != nil {
		status = "已识别"
	}
	return fmt.Sprintf("%s · %s · %s", rec.Created.Format("01-02 15:04"), formatSecs(rec.Duration), status)
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
			ui.Icon(c, iconCheck).FontSize(13).TextColor(ui.Hex("#ffffff"))
		})
	}
}

// icon parses a 24×24 stroked icon drawn in the current text color.
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconCheck  = icon(`<path d="M20 6 9 17l-5-5"/>`)
	iconPlay   = icon(`<path d="M6 4l14 8-14 8z" fill="currentColor" stroke="none"/>`)
	iconPause  = icon(`<path d="M7 4v16M17 4v16"/>`)
	iconText   = icon(`<path d="M4 6h16M4 12h16M4 18h10"/>`)
	iconFolder = icon(`<path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2z"/>`)
	iconGear   = icon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>`)
	iconPlus   = icon(`<path d="M12 5v14M5 12h14"/>`)
	iconTrash  = icon(`<path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6M10 11v6M14 11v6"/>`)
)

func playIcon(playing bool) *ui.SVG {
	if playing {
		return iconPause
	}
	return iconPlay
}
