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
		ui.Column(c).Width(360).Gap(16).Children(func() {
			ui.Text(c, "重命名录音").FontSize(16).Bold().TextColor(ink)
			name := a.renameRecName
			ti := ui.TextInput(c.Key("rec-name"), &name).FontSize(14).TextColor(ink)
			ti.Placeholder("录音名称")
			if ti.Changed() {
				a.renameRecName = name
			}
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				cancel := ui.ButtonBase(c.Key("rec-rename-cancel")).Padding(8, 16).Radius(8).Center().Background(cardBg2).Border(1, line)
				cancel.Children(func() { ui.Text(c, "取消").FontSize(13).TextColor(ink) })
				if cancel.Clicked() {
					a.renameRecOpen = false
				}
				save := ui.ButtonBase(c.Key("rec-rename-save")).Padding(8, 16).Radius(8).Center().Background(accent)
				save.Children(func() { ui.Text(c, "保存").FontSize(13).Bold().TextColor(onAccent) })
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

// dropAudioConfirm confirms throwing away a recording's audio while keeping
// its text.
//
// Without a transcript there is no text to keep, so that case says plainly
// what is about to be lost and points at the other delete — the one that
// takes the empty entry with it — rather than letting the user end up with a
// row that holds nothing and cannot be transcribed again.
func (a *app) dropAudioConfirm(c *ui.Context) {
	if !a.confirmDelAudio {
		return
	}
	rec, ok := a.lib.Get(a.delAudioID)
	if !ok {
		a.confirmDelAudio, a.delAudioID = false, ""
		return
	}
	body := "音频文件移到废纸篓，转写、摘要、脑图、笔记和问答都保留。"
	if rec.Transcript == nil {
		body = "这条录音还没转写成文字。删掉音频就再也转写不了，条目里将什么都不剩——" +
			"如果确实不再需要它，请改用「删除条目」。"
	}
	if i := ui.AlertDialog(c.Key("delaudioq"), &a.confirmDelAudio, "删除音频文件？", body, "删除音频", "取消"); i == 0 {
		a.dropAudio(rec)
		a.delAudioID = ""
	}
}
