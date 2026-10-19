package app

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/store"
)

func (a *app) view(c *ui.Context) {
	// The framework has already picked the light or dark theme for the
	// appearance in force, following the system when the settings ask it
	// to; useAppearance takes that choice and paints the app's own colors
	// over it.
	t := *c.Theme()
	useAppearance(&t)
	c.SetTheme(&t)

	if ok, reason := a.rec.Available(); !ok {
		ui.Column(c).Fill().Center().Padding(40).Gap(10).Background(pageBg).Children(func() {
			ui.Text(c, "无法在此设备录制").FontSize(18).Bold().TextColor(ink)
			ui.Text(c, reason).FontSize(13).TextColor(muted)
		})
		return
	}
	root := ui.Row(c).Fill().AlignItems(ui.Stretch).Background(pageBg)
	root.Children(func() {
		a.sidebar(c)
		a.detail(c)
	})
	// The dialog is capped by the window, so its scrolling body gets what is
	// left over. root's box is the window's content area — the previous
	// frame's, which is all a dialog needs and keeps this out of the window
	// API.
	a.settingsModal(c, root.Bounds().H)
	// The row menu's dialogs live at the root, not in the sidebar row: a
	// right-click can target a row that is scrolled away or rebuilt by the
	// time the dialog's button is clicked.
	a.renameRecModal(c)
	a.deleteConfirm(c)
	a.dropAudioConfirm(c)
	a.importFailureDialog(c)
}

func (a *app) sidebar(c *ui.Context) {
	ui.Column(c).Width(280).Shrink(0).Background(cardBg).Children(func() {
		ui.Column(c).Padding(20, 20, 16).Gap(12).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				// The app's name, where the list it heads used to be named
				// after itself: the recordings below are plainly that, and
				// the window's own title is only "Voxi".
				ui.Text(c, "有听有记 Voxi").FontSize(20).Bold().TextColor(ink).Grow(1).SingleLine()
				a.importButton(c)
				// Both are icon-only, so the tip is the only thing that says
				// what they do; it arrives once the pointer rests on them.
				gear := ui.ButtonBase(c.Key("open-settings")).Padding(6).Radius(8).Center().Tooltip("设置")
				gear.Background(controlFill(gear, cardBg2, cardBg3))
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
			ui.Column(c).Grow(1).Center().Padding(20).Gap(12).Children(func() {
				ui.Text(c, "还没有录音").FontSize(13).TextColor(muted)
				// An empty library is where an import is most likely to be
				// what the user wants, and the icon in the header is easy to
				// miss before there is anything to look at.
				imp := ui.ButtonBase(c.Key("import-empty")).Padding(8, 16).Radius(8).Center().Border(1, btnStroke)
				imp.Background(controlFill(imp, cardBg2, cardBg3))
				imp.Children(func() { ui.Text(c, "导入音频文件").FontSize(13).TextColor(ink) })
				if imp.Clicked() {
					a.importAudio()
				}
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
		// A tinted fill, not the accent border it used to share with every
		// primary button: a selection is a place, not an action. Hover owns
		// cardBg2, so the two no longer look alike — the old selected row was
		// cardBg2 too, told apart only by a thin blue edge.
		row.Background(selBg)
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
		ui.Text(c, recMetaStatus(rec, a.asrStatus(rec.ID))).FontSize(11).TextColor(muted)
	})
	if row.Clicked() {
		a.selectRec(rec.ID)
	}
	// A row's actions live on its context menu rather than in the detail
	// pane's action bar, which a right-clicked row is not always the one
	// showing.
	//
	// Two deletes, named for what they take: the audio alone, or the whole
	// entry. Only one of them is offered once the audio is gone, so the
	// menu never leads to a dialog that can only say no.
	audio := hasAudio(rec)
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("重命名").Chosen() {
			a.renameRecOpen, a.renameRecID, a.renameRecName = true, rec.ID, rec.Name
		}
		if audio && m.Item("在访达中显示").Chosen() {
			mygo.Shell.ShowItemInFolder(rec.File)
		}
		m.Separator()
		if audio && m.Item("删除音频文件（保留文字）").Chosen() {
			a.delAudioID, a.confirmDelAudio = rec.ID, true
		}
		if m.Item("删除条目").Chosen() {
			a.delRecID, a.confirmDel = rec.ID, true
		}
	})
}
