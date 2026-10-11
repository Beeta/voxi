package main

import "github.com/egoist/mygo/ui"

// notesView is the free-form note editor for a recording. Edits save on change.
func (a *app) notesView(c *ui.Context, rec Recording) {
	ui.Column(c).Grow(1).Padding(20, 32).Children(func() {
		notes := rec.Notes
		ta := ui.TextArea(c.Key("notes-"+rec.ID), &notes).Grow(1).FontSize(14).TextColor(ink)
		ta.Placeholder("在这里记笔记……支持多行，自动保存。")
		if ta.Changed() {
			a.store.setNotes(rec.ID, notes)
		}
	})
}
