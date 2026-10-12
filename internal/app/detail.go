package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/store"
)

func (a *app) detail(c *ui.Context) {
	a.mu.Lock()
	id := a.selected
	a.mu.Unlock()
	rec, ok := a.lib.Get(id)
	col := ui.Column(c).Grow(1).MinWidth(0).Background(pageBg)
	if !ok {
		col.Center().Padding(40).Children(func() {
			ui.Text(c, "选择左侧的录音，或开始一段新录制").FontSize(14).TextColor(muted)
		})
		return
	}
	col.Children(func() {
		ui.Column(c).Padding(28, 32, 14).Gap(6).Children(func() {
			a.titleRow(c, rec)
			ui.Text(c, recMeta(rec)).FontSize(12).TextColor(muted)
		})
		ui.Column(c).Padding(0, 32, 16).Children(func() {
			a.playerBar(c, rec)
		})
		ui.Column(c).Padding(0, 32).Children(func() {
			ui.Tabs(c.Key("tab"), &a.tab, "转写", "摘要", "脑图", "问答", "笔记")
		})
		ui.Column(c).Height(1).Background(line)
		ui.Column(c).Grow(1).Children(func() {
			switch a.tab {
			case 1:
				a.scrollPage(c, "sum", func() { a.summaryView(c, rec) })
			case 2:
				a.scrollPage(c, "mind", func() { a.mindmapView(c, rec) })
			case 3:
				a.qaView(c, rec)
			case 4:
				a.notesView(c, rec)
			default:
				a.scrollPage(c, "tr", func() { a.transcriptView(c, rec) })
			}
		})
	})
}

// scrollPage wraps a tab body in a padded vertical scroll area.
func (a *app) scrollPage(c *ui.Context, key string, body func()) {
	ui.Scroll(c.Key(key)).Grow(1).Children(func() {
		ui.Column(c).Padding(20, 32).Children(func() { body() })
	})
}

func (a *app) titleRow(c *ui.Context, rec store.Recording) {
	name := rec.Name
	ed := ui.EditableText(c.Key("name-"+rec.ID), &name).FontSize(22).Bold().TextColor(ink)
	if ed.Changed() {
		a.lib.Rename(rec.ID, name)
	}
}
