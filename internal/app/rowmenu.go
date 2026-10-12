package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// renameRecModal renames a recording chosen from a row's context menu. The
// detail pane's title is editable too, but a row can be right-clicked
// without being selected, so this edits the menu's target instead.
func (a *app) renameRecModal(c *ui.Context) {
	if !a.renameRecOpen {
		return
	}
	ui.Modal(c.Key("rec-rename"), &a.renameRecOpen, func() {
		ui.Column(c).Width(360).Gap(14).Children(func() {
			ui.Text(c, "重命名录音").FontSize(16).Bold().TextColor(ink)
			name := a.renameRecName
			ti := ui.TextInput(c.Key("rec-name"), &name).FontSize(14).TextColor(ink)
			ti.Placeholder("录音名称")
			if ti.Changed() {
				a.renameRecName = name
			}
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				cancel := ui.ButtonBase(c.Key("rec-rename-cancel")).Padding(8, 14).Radius(8).Center().Background(cardBg2).Border(1, line)
				cancel.Children(func() { ui.Text(c, "取消").FontSize(13).TextColor(ink) })
				if cancel.Clicked() {
					a.renameRecOpen = false
				}
				save := ui.ButtonBase(c.Key("rec-rename-save")).Padding(8, 14).Radius(8).Center().Background(accent)
				save.Children(func() { ui.Text(c, "保存").FontSize(13).Bold().TextColor(ui.Hex("#ffffff")) })
				if save.Clicked() || ti.Submitted() {
					a.applyRecRename()
				}
			})
		})
	})
}

// applyRecRename writes the drafted name through the store and closes the
// modal. An all-whitespace name is dropped: the row would have no label to
// right-click next time.
func (a *app) applyRecRename() {
	if name := strings.TrimSpace(a.renameRecName); name != "" {
		a.lib.Rename(a.renameRecID, name)
	}
	a.renameRecOpen = false
}

// deleteConfirm confirms the delete chosen from a row's context menu.
func (a *app) deleteConfirm(c *ui.Context) {
	if i := ui.AlertDialog(c.Key("delq"), &a.confirmDel, "删除录音", "将把它移到废纸篓。", "删除", "取消"); i == 0 {
		if rec, ok := a.lib.Get(a.delRecID); ok {
			a.doDelete(rec)
		}
		a.delRecID = ""
	}
}
