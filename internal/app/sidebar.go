package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/zhaorongcun/voxi/internal/store"
)

func (a *app) view(c *ui.Context) {
	t := *c.Theme()
	t.Dark = true
	t.Background, t.Surface, t.Border = pageBg, cardBg, line
	t.Text, t.TextMuted, t.Accent = ink, muted, accent
	t.SurfaceHover, t.SurfacePressed = cardBg2, cardBg2
	c.SetTheme(&t)

	if ok, reason := a.rec.Available(); !ok {
		ui.Column(c).Fill().Center().Padding(40).Gap(10).Background(pageBg).Children(func() {
			ui.Text(c, "无法在此设备录制").FontSize(18).Bold().TextColor(ink)
			ui.Text(c, reason).FontSize(13).TextColor(muted)
		})
		return
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(pageBg).Children(func() {
		a.sidebar(c)
		a.detail(c)
	})
	a.settingsModal(c)
}

func (a *app) sidebar(c *ui.Context) {
	ui.Column(c).Width(280).Shrink(0).Background(cardBg).Children(func() {
		ui.Column(c).Padding(20, 20, 14).Gap(12).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "录音").FontSize(20).Bold().TextColor(ink).Grow(1)
				gear := ui.ButtonBase(c.Key("open-settings")).Padding(6).Radius(8).Center().Background(cardBg2)
				gear.Children(func() { ui.Icon(c, iconGear).FontSize(16).TextColor(muted) })
				if gear.Clicked() {
					a.showSettings = true
				}
			})
			a.sourceSummary(c)
			a.recordControl(c)
		})
		ui.Column(c).Height(1).Background(line)
		recs := a.lib.Snapshot()
		if len(recs) == 0 {
			ui.Column(c).Grow(1).Center().Padding(20).Children(func() {
				ui.Text(c, "还没有录音").FontSize(13).TextColor(muted)
			})
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(10).Gap(6).Children(func() {
				for i := range recs {
					a.fileRow(c, recs[i])
				}
			})
		})
	})
}

// sourceSummary shows which capture sources are enabled; they are configured
// in the settings dialog.
func (a *app) sourceSummary(c *ui.Context) {
	s := a.settings.snapshot()
	var parts []string
	if s.Mic {
		parts = append(parts, labelMic)
	}
	if s.System {
		parts = append(parts, labelSystem)
	}
	txt, col := "音频源："+strings.Join(parts, " · "), muted
	if len(parts) == 0 {
		txt, col = "未选择音频源，点右上角齿轮设置", recRed
	}
	ui.Text(c, txt).FontSize(12).TextColor(col)
}

// chosen counts the sources whose check boxes are on.
func (a *app) chosen() int {
	s := a.settings.snapshot()
	n := 0
	if s.Mic {
		n++
	}
	if s.System {
		n++
	}
	return n
}

func (a *app) fileRow(c *ui.Context, rec store.Recording) {
	a.mu.Lock()
	selected := a.selected == rec.ID
	a.mu.Unlock()
	row := ui.Column(c.Key(rec.ID)).Padding(10, 12).Gap(4).Radius(8).Cursor(ui.CursorPointer)
	switch {
	case selected:
		row.Background(cardBg2).Border(1, accent)
	case row.Hovered():
		row.Background(cardBg2)
	default:
		row.Border(1, line)
	}
	row.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, rec.Name).FontSize(14).Bold().TextColor(ink).Grow(1).MinWidth(0)
			if rec.Transcript != nil {
				ui.Icon(c, iconText).FontSize(13).TextColor(okGreen)
			}
		})
		ui.Text(c, recMeta(rec)).FontSize(11).TextColor(muted)
	})
	if row.Clicked() {
		a.selectRec(rec.ID)
	}
}
