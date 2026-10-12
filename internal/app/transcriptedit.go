package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/store"
)

// segmentRow renders one transcript segment: a clickable speaker chip, a
// clickable timestamp (seeks), and the text — click it to edit inline, which
// auto-saves. The segment being edited is tracked by a.editSeg.
func (a *app) segmentRow(c *ui.Context, rec store.Recording, i int, seg store.Segment, active bool) {
	row := ui.Row(c.Key(i)).Padding(8, 10).Gap(12).Radius(6).AlignItems(ui.Start)
	switch {
	case active:
		row.Background(cardBg2)
	case row.Hovered():
		row.Background(cardBg)
	}
	row.Children(func() {
		if seg.Speaker != "" {
			sp := ui.Column(c.Key("spk")).Padding(2, 6).Radius(6).Background(cardBg2).Border(1, line).Cursor(ui.CursorPointer)
			sp.Children(func() { ui.Text(c, seg.Speaker).FontSize(12).Bold().TextColor(muted) })
			if sp.Clicked() {
				a.renaming, a.renameIdx, a.renameDraft = true, i, seg.Speaker
			}
		}
		tc := muted
		if active {
			tc = accent
		}
		ts := ui.Column(c.Key("ts")).Cursor(ui.CursorPointer)
		ts.Children(func() { ui.Text(c, formatSecs(seg.Start)).FontSize(12).Bold().TextColor(tc).Width(46) })
		if ts.Clicked() {
			a.seekTo(seg.Start)
		}
		if a.editSeg == i {
			txt := seg.Text
			ta := ui.TextArea(c.Key("edit"), &txt).Lines(1, 6).Grow(1).FontSize(14).TextColor(ink)
			if ta.Changed() {
				a.lib.SetSegmentText(rec.ID, i, txt)
			}
			done := ui.ButtonBase(c.Key("done")).Padding(4, 10).Radius(6).Center().Background(cardBg2)
			done.Children(func() { ui.Text(c, "完成").FontSize(12).Bold().TextColor(accent) })
			if done.Clicked() {
				a.editSeg = -1
			}
		} else {
			tx := ui.Column(c.Key("txt")).Grow(1).Cursor(ui.CursorPointer)
			tx.Children(func() { ui.Text(c, seg.Text).FontSize(14).TextColor(ink) })
			if tx.Clicked() {
				a.editSeg = i
			}
		}
	})
}

// speakerRenameModal renames a segment's speaker, applying it to just that
// segment or to all segments currently sharing the same speaker name.
func (a *app) speakerRenameModal(c *ui.Context, rec store.Recording) {
	if !a.renaming {
		return
	}
	ui.Modal(c.Key("spk-rename"), &a.renaming, func() {
		ui.Column(c).Width(360).Gap(14).Children(func() {
			ui.Text(c, "重命名说话人").FontSize(16).Bold().TextColor(ink)
			name := a.renameDraft
			ti := ui.TextInput(c.Key("spk-name"), &name).FontSize(14).TextColor(ink)
			ti.Placeholder("说话人名称")
			if ti.Changed() {
				a.renameDraft = name
			}
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				one := ui.ButtonBase(c.Key("one")).Padding(8, 14).Radius(8).Center().Background(cardBg2).Border(1, line)
				one.Children(func() { ui.Text(c, "仅此条").FontSize(13).TextColor(ink) })
				if one.Clicked() {
					a.applyRename(rec, false)
				}
				all := ui.ButtonBase(c.Key("all")).Padding(8, 14).Radius(8).Center().Background(accent)
				all.Children(func() { ui.Text(c, "全部相同").FontSize(13).Bold().TextColor(onAccent) })
				if all.Clicked() {
					a.applyRename(rec, true)
				}
			})
		})
	})
}

// applyRename writes the drafted speaker name through the store and closes the
// modal.
func (a *app) applyRename(rec store.Recording, global bool) {
	a.lib.RenameSpeaker(rec.ID, a.renameIdx, a.renameDraft, global)
	a.renaming = false
}
