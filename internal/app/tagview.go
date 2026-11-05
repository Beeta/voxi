package app

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/store"
)

// tagFilter is the sidebar's tag drop-down over the list: 全部 or one tag,
// each with how many recordings it holds. A tag's option is managed from
// its right-click menu (编辑, 删除), and 新建标签 sits at the foot of the
// popup, where it is found without knowing to right-click.
//
// It is a popover of its own rather than a SelectBase: choosing 编辑 from
// an option's menu has to close the popup for the dialog, and a select
// keeps its open state to itself.
func (a *app) tagFilter(c *ui.Context, set Settings, recs []store.Recording, filter string) {
	counts := tagCounts(recs)
	label := "全部录音"
	n := len(recs)
	if t, ok := tagByID(set.Tags, filter); ok {
		label, n = t.Name, counts[t.ID]
	}
	trig := ui.ButtonBase(c.Key("tag-filter")).Padding(6, 10).Gap(6).Radius(8).Border(1, line).Label("标签筛选")
	trig.Background(controlFill(trig, cardBg2, cardBg3))
	trig.Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(ink).Grow(1).MinWidth(0).SingleLine()
		ui.Text(c, strconv.Itoa(n)).FontSize(12).TextColor(muted)
		ui.Icon(c, iconChevronDown).FontSize(13).TextColor(muted)
	})
	if trig.Clicked() {
		a.tagMenuOpen = !a.tagMenuOpen
	}
	ui.PopoverBase(c, trig, &a.tagMenuOpen, func(panel ui.Element) {
		panel.Margin(4, 0, 0, 0).MinWidth(trig.Bounds().W).Padding(4).Gap(2).Radius(10).
			Background(cardBg).Border(1, line).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
		a.tagOption(c, "", "全部录音", len(recs), filter == "")
		for _, t := range set.Tags {
			opt := a.tagOption(c, t.ID, t.Name, counts[t.ID], filter == t.ID)
			opt.ContextMenu(func(m *ui.Menu) {
				if m.Item("编辑").Chosen() {
					a.openTagEditor(t.ID, "")
				}
				if m.Item("删除").Chosen() {
					a.tagMenuOpen, a.delTagID, a.confirmDelTag = false, t.ID, true
				}
			})
		}
		ui.Column(c).Height(1).Background(line).Margin(2, 0)
		add := ui.ButtonBase(c.Key("tag-new")).Padding(6, 10).Gap(6).Radius(6).Justify(ui.Start)
		add.Background(controlFill(add, cardBg, cardBg2))
		add.Children(func() {
			ui.Icon(c, iconPlus).FontSize(12).TextColor(accent)
			ui.Text(c, "新建标签…").FontSize(13).TextColor(accent)
		})
		if add.Clicked() {
			a.openTagEditor("", "")
		}
	})
}

// tagOption is one option of the tag popup: choosing it filters by tag id,
// "" for all, and closes the popup. It returns the option for its menu.
func (a *app) tagOption(c *ui.Context, id, name string, n int, chosen bool) ui.Element {
	// The label tells the option from the chips of the same name on the
	// rows under the popup, for assistive technology as for the tests.
	opt := ui.ButtonBase(c.Key("tag-opt-"+id)).Padding(6, 10).Gap(8).Radius(6).Justify(ui.Start).Label("筛选：" + name)
	switch {
	case chosen:
		opt.Background(selBg)
	default:
		opt.Background(controlFill(opt, cardBg, cardBg2))
	}
	opt.Children(func() {
		ui.Text(c, name).FontSize(13).TextColor(ink).Grow(1).MinWidth(0).SingleLine()
		ui.Text(c, strconv.Itoa(n)).FontSize(12).TextColor(muted)
	})
	if opt.Clicked() {
		a.tagMenuOpen = false
		a.setFilter(id)
	}
	return opt
}

// recTagChips shows the names of a recording's tags under its row's meta
// line, so what a row carries shows without opening its menu.
func recTagChips(c *ui.Context, rec store.Recording, tags []Tag) {
	var names []string
	for _, id := range rec.Tags {
		if t, ok := tagByID(tags, id); ok {
			names = append(names, t.Name)
		}
	}
	if len(names) == 0 {
		return
	}
	ui.Row(c).Wrap().Gap(4).Children(func() {
		for _, name := range names {
			ui.Column(c).Padding(1, 6).Radius(999).Background(cardBg3).Children(func() {
				ui.Text(c, name).FontSize(10).TextColor(muted).NoWrap()
			})
		}
	})
}

// tagSubmenu is the 标签 submenu of a recording row's menu: every tag, a
// check by those the recording carries, choosing one puts it on or takes
// it off; and 新建标签, which makes a tag and puts it on this recording.
func (a *app) tagSubmenu(m *ui.Menu, rec store.Recording, tags []Tag) {
	m.Submenu("标签", func(sm *ui.Menu) {
		for _, t := range tags {
			on := false
			for _, id := range rec.Tags {
				on = on || id == t.ID
			}
			if sm.Item(t.Name).Checked(on).Chosen() {
				a.lib.SetTagged(rec.ID, t.ID, !on)
			}
		}
		if len(tags) > 0 {
			sm.Separator()
		}
		if sm.Item("新建标签…").Chosen() {
			a.openTagEditor("", rec.ID)
		}
	})
}

// tagDialogs draws the tag editor and the delete confirmation, at the root
// of the view: both are opened from a popup or a menu that is gone by the
// time their buttons are clicked.
func (a *app) tagDialogs(c *ui.Context) {
	if a.tagEditOpen {
		title := "编辑标签"
		if a.tagEditID == "" {
			title = "新建标签"
		}
		ui.Modal(c.Key("tag-edit"), &a.tagEditOpen, func() {
			ui.Column(c).Width(360).Gap(12).Children(func() {
				ui.Text(c, title).FontSize(16).Bold().TextColor(ink)
				ti := ui.TextInput(c.Key("tag-name"), &a.tagEditName).FontSize(14).TextColor(ink)
				ti.Label("标签名称").Placeholder("标签名称").AutoFocus()
				ti.Changed()
				name := tagName(a.tagEditName)
				taken := name != "" && tagTaken(a.settings.snapshot().Tags, a.tagEditID, name)
				if taken {
					ui.Text(c, "已有同名标签").FontSize(12).TextColor(recRed)
				}
				ok := name != "" && !taken
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Spacer(c)
					cancel := ui.ButtonBase(c.Key("tag-edit-cancel")).Padding(8, 16).Radius(8).Center().Background(cardBg2).Border(1, line)
					cancel.Children(func() { ui.Text(c, "取消").FontSize(13).TextColor(ink) })
					if cancel.Clicked() {
						a.tagEditOpen = false
					}
					save := ui.ButtonBase(c.Key("tag-edit-save")).Padding(8, 16).Radius(8).Center().Background(accent).Disabled(!ok)
					fg := onAccent
					if !ok {
						save.Background(cardBg2)
						fg = muted
					}
					save.Children(func() { ui.Text(c, "保存").FontSize(13).Bold().TextColor(fg) })
					if ok && (save.Clicked() || ti.Submitted()) && a.saveTag() {
						a.tagEditOpen = false
					}
				})
			})
		})
	}
	if !a.confirmDelTag {
		return
	}
	t, ok := tagByID(a.settings.snapshot().Tags, a.delTagID)
	if !ok {
		a.confirmDelTag, a.delTagID = false, ""
		return
	}
	n := tagCounts(a.lib.Snapshot())[t.ID]
	body := fmt.Sprintf("「%s」将被删除，并从 %d 条录音上去掉。录音本身不受影响。", t.Name, n)
	if i := ui.AlertDialog(c.Key("tag-delq"), &a.confirmDelTag, "删除标签？", body, "删除标签", "取消"); i == 0 {
		a.deleteTag(t.ID)
		a.delTagID = ""
	}
}
