package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/store"
)

// segmentRow renders one transcript segment: a clickable speaker chip, a
// timestamp (click it to seek, when there is audio to seek in), and the text
// — click it to edit inline, which auto-saves. The segment being edited is
// tracked by a.editSeg. tint is the fill this speaker's chip is drawn in;
// seekable is whether the recording still has audio for the timestamp to
// jump into.
func (a *app) segmentRow(c *ui.Context, rec store.Recording, i int, seg store.Segment, active, seekable bool, tint ui.Color) {
	row := ui.Row(c.Key(i)).Padding(8, 10).Gap(12).Radius(8).AlignItems(ui.Start)
	switch {
	case active:
		row.Background(cardBg2)
	case row.Hovered():
		row.Background(cardBg)
	}
	row.Children(func() {
		if seg.Speaker != "" {
			// Shrink(0) keeps the chip its own width in a row this crowded,
			// and NoWrap keeps the name one piece. SingleLine would do the
			// second half too, but by cutting the name short with an
			// ellipsis: a chip measures a hair narrower than the text it
			// holds lays out, and "说话人1" is exactly at that boundary, so
			// it came out "说话…".
			sp := ui.Column(c.Key("spk")).Shrink(0).Padding(2, 7).Radius(6).Background(tint).Cursor(ui.CursorPointer)
			sp.Children(func() { ui.Text(c, seg.Speaker).FontSize(11).Bold().TextColor(ink).NoWrap() })
			if sp.Clicked() {
				a.renaming, a.renameIdx, a.renameDraft = true, i, seg.Speaker
			}
		}
		tc := muted
		if active {
			tc = accent
		}
		ts := ui.Column(c.Key("ts"))
		if seekable {
			ts.Cursor(ui.CursorPointer)
		}
		ts.Children(func() { ui.Text(c, formatSecs(seg.Start)).FontSize(12).Bold().TextColor(tc).Width(46) })
		if seekable && ts.Clicked() {
			a.seekTo(seg.Start)
		}
		if a.editSeg == i {
			txt := seg.Text
			// MinHeight lifts the floor a text area carries (three lines), so
			// the box is as tall as the text in it — one line, for most
			// segments — and grows with what is typed.
			ta := ui.TextArea(c.Key("edit"), &txt).Lines(1, 6).MinHeight(0).Grow(1).FontSize(14).TextColor(ink)
			ta.AutoFocus()
			if ta.Changed() {
				a.lib.SetSegmentText(rec.ID, i, txt)
			}
			// There is nothing to confirm: the text is saved as it is typed,
			// so the edit ends when the editor lets go of the focus — a click
			// elsewhere, or Tab away — or on Escape. That is what the 完成
			// button used to be for, and it cost a click and a whole column
			// of the row.
			if !ta.FocusWithin() || ta.Shortcut(0, ui.KeyEscape) {
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
		ui.Column(c).Width(360).Gap(16).Children(func() {
			ui.Text(c, "重命名说话人").FontSize(16).Bold().TextColor(ink)
			name := a.renameDraft
			ti := ui.TextInput(c.Key("spk-name"), &name).FontSize(14).TextColor(ink)
			ti.Placeholder("说话人名称")
			if ti.Changed() {
				a.renameDraft = name
			}
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				one := ui.ButtonBase(c.Key("one")).Padding(8, 16).Radius(8).Center().Background(cardBg2).Border(1, line)
				one.Children(func() { ui.Text(c, "仅此条").FontSize(13).TextColor(ink) })
				if one.Clicked() {
					a.applyRename(rec, false)
				}
				all := ui.ButtonBase(c.Key("all")).Padding(8, 16).Radius(8).Center().Background(accent)
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
