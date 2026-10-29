package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
)

// recordControl is the sidebar's record widget: a start button, or while a
// capture is in flight the live timer with pause / resume and stop, a
// spinner, or a startup error.
func (a *app) recordControl(c *ui.Context) {
	st, elapsed, _, err := a.rec.Snapshot()
	a.mu.Lock()
	recording := a.recording
	warn := a.recWarn
	a.mu.Unlock()
	switch {
	case st == audio.StateRecording || st == audio.StatePaused:
		paused := st == audio.StatePaused
		// Paused, the red goes: the border and the timer turn muted, and the
		// line says outright that nothing is being taken in, since the
		// system's recording indicator stays on through a pause.
		border, clock, title := recRed, recRed, "录制中"
		if paused {
			border, clock, title = line, muted, "已暂停"
		}
		ui.Column(c).Padding(12).Gap(10).Radius(10).Background(cardBg2).Border(1, border).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(13).Bold().TextColor(ink).Grow(1)
				ui.Text(c, formatClock(elapsed)).FontSize(15).Bold().TextColor(clock)
			})
			if paused {
				ui.Text(c, "暂停中不录入声音（系统录制指示仍会亮着）").FontSize(11).TextColor(muted)
			}
			if warn != "" {
				ui.Text(c, warn).FontSize(11).TextColor(recRed)
			}
			ui.Row(c).Gap(8).Children(func() {
				label := "暂停"
				if paused {
					label = "继续"
				}
				pr := ui.ButtonBase(c.Key("pause")).Padding(10, 12).Radius(8).Center().Border(1, btnStroke).Grow(1)
				pr.Background(controlFill(pr, cardBg, cardBg3))
				pr.Children(func() { ui.Text(c, label).FontSize(13).Bold().TextColor(ink) })
				if pr.Clicked() {
					if paused {
						a.rec.Resume()
					} else {
						a.rec.Pause()
					}
				}
				stop := ui.ButtonBase(c.Key("stop")).Padding(10, 12).Radius(8).Center().Background(recRed).Grow(1)
				stop.Children(func() { ui.Text(c, "停止并保存").FontSize(13).Bold().TextColor(onAccent) })
				if stop.Clicked() {
					a.stopRecording()
				}
			})
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
		btn := ui.ButtonBase(c.Key("rec")).Padding(12, 16).Radius(10).Center().Border(1, stroke).Disabled(!canRec)
		// Hovered is false while the button is disabled, so the fill only
		// steps up when the click would do something.
		btn.Background(controlFill(btn, cardBg2, cardBg3))
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
