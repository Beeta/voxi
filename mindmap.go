package main

import "github.com/egoist/mygo/ui"

// mindmapView is the mind-map tab: a generate button, then the outline tree
// rendered as collapsible indented rows.
func (a *app) mindmapView(c *ui.Context, rec Recording) {
	a.mu.Lock()
	busy := a.mindmapping[rec.ID]
	a.mu.Unlock()
	hasTranscript := rec.Transcript != nil
	ui.Column(c).Gap(14).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, "脑图大纲").FontSize(13).Bold().TextColor(muted).Grow(1)
			if !busy && hasTranscript {
				label := "生成脑图"
				if rec.Mindmap != nil {
					label = "重新生成"
				}
				b := ui.ButtonBase(c.Key("gen-mind")).Padding(8, 14).Radius(8).Center().Background(cardBg2).Border(1, accent)
				b.Children(func() { ui.Text(c, label).FontSize(12).Bold().TextColor(accent) })
				if b.Clicked() {
					a.generateMindmap(rec)
				}
			}
		})
		switch {
		case busy:
			loadingRow(c, "正在生成脑图…")
		case !hasTranscript:
			ui.Text(c, "请先在「转写」页签完成识别。").FontSize(13).TextColor(muted)
		case rec.Mindmap == nil || rec.Mindmap.Root == nil:
			ui.Text(c, "还没有脑图。点击「生成脑图」。").FontSize(13).TextColor(muted)
		default:
			if rec.Mindmap.Stale {
				ui.Text(c, "⚠ 转写已更新，脑图可能过期，建议重新生成。").FontSize(12).TextColor(recRed)
			}
			a.mindNode(c, rec.Mindmap.Root, "")
		}
	})
}

// mindNode renders one outline node and, when expanded, its children indented
// below. path is a stable key for the per-node collapse state.
func (a *app) mindNode(c *ui.Context, n *MindNode, path string) {
	key := path + "/" + n.Text
	hasKids := len(n.Children) > 0
	open := !a.expanded[key] // zero value (false) = expanded
	ui.Column(c.Key(key)).Gap(4).Children(func() {
		row := ui.Row(c).Gap(8).AlignItems(ui.Start).Padding(4, 0).Children(func() {
			mark, mc := "·", accent
			if hasKids {
				mark, mc = triangle(open), muted
			}
			ui.Text(c, mark).FontSize(13).TextColor(mc).Width(16)
			ui.Text(c, n.Text).FontSize(14).TextColor(ink).Grow(1).Selectable()
		})
		if hasKids && row.Clicked() {
			a.expanded[key] = open
		}
		if hasKids && open {
			ui.Column(c).Padding(0, 0, 0, 20).Gap(4).Children(func() {
				for _, ch := range n.Children {
					a.mindNode(c, ch, key)
				}
			})
		}
	})
}

func triangle(open bool) string {
	if open {
		return "▾"
	}
	return "▸"
}
