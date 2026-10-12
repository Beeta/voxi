package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
)

// recordControl is the sidebar's record widget: a start button, or while a
// capture is in flight the live timer with a stop button, a spinner, or a
// startup error.
func (a *app) recordControl(c *ui.Context) {
	st, elapsed, _, err := a.rec.Snapshot()
	a.mu.Lock()
	recording := a.recording
	a.mu.Unlock()
	switch {
	case st == audio.StateRecording:
		ui.Column(c).Padding(12).Gap(10).Radius(10).Background(cardBg2).Border(1, recRed).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "录制中").FontSize(13).Bold().TextColor(ink).Grow(1)
				ui.Text(c, formatClock(elapsed)).FontSize(15).Bold().TextColor(recRed)
			})
			stop := ui.ButtonBase(c.Key("stop")).Padding(10, 14).Radius(8).Center().Background(recRed)
			stop.Children(func() { ui.Text(c, "停止并保存").FontSize(13).Bold().TextColor(ui.Hex("#ffffff")) })
			if stop.Clicked() {
				a.stopRecording()
			}
		})
	case st == audio.StateStarting || st == audio.StateStopping:
		ui.Row(c).Padding(12).Gap(10).Radius(10).Center().Background(cardBg2).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "处理中…").FontSize(13).TextColor(muted)
		})
	case recording && err != nil:
		ui.Column(c).Padding(12).Gap(8).Radius(10).Background(cardBg2).Border(1, recRed).Children(func() {
			ui.Textf(c, "录制失败：%s", err.Error()).FontSize(12).TextColor(recRed)
			cl := ui.ButtonBase(c.Key("close")).Padding(8, 12).Radius(8).Center().Background(cardBg)
			cl.Children(func() { ui.Text(c, "关闭").FontSize(12).TextColor(ink) })
			if cl.Clicked() {
				a.mu.Lock()
				a.recording = false
				a.mu.Unlock()
			}
		})
	default:
		canRec := a.chosen() > 0
		// Idle start button stays a neutral button so it never reads as a
		// recording in progress; only the dot carries the record red. A
		// disabled one keeps the dim separator outline, so it does not
		// invite a click it cannot take.
		stroke := btnStroke
		if !canRec {
			stroke = line
		}
		btn := ui.ButtonBase(c.Key("rec")).Padding(12, 16).Radius(10).Center().Background(cardBg2).Border(1, stroke).Disabled(!canRec)
		btn.Children(func() {
			fg, dot := ink, recRed
			if !canRec {
				fg, dot = muted, muted
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Center().Children(func() {
				ui.Column(c).Size(10, 10).Radius(999).Background(dot)
				ui.Text(c, "开始录制").FontSize(14).Bold().TextColor(fg)
			})
		})
		if canRec && btn.Clicked() {
			a.startRecording()
		}
	}
}
