package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/store"
)

// mindmapView is the mind-map tab: the outline tree rendered as collapsible
// indented rows. The generate button is in the player bar (mindmapAction).
func (a *app) mindmapView(c *ui.Context, rec store.Recording) {
	a.mu.Lock()
	busy := a.mindmapping[rec.ID]
	a.mu.Unlock()
	ui.Column(c).Gap(16).Children(func() {
		switch {
		case busy:
			loadingRow(c, "正在生成脑图…")
		case rec.Transcript == nil:
			ui.Text(c, "请先在「转写」页完成识别，再点底部的「生成脑图」。").FontSize(13).TextColor(muted)
		case rec.Mindmap == nil || rec.Mindmap.Root == nil:
			ui.Text(c, "还没有脑图。点底部的「生成脑图」。").FontSize(13).TextColor(muted)
		default:
			if rec.Mindmap.Stale {
				ui.Text(c, "⚠ 转写已更新，脑图可能过期，建议重新生成。").FontSize(12).TextColor(recRed)
			}
			a.mindNode(c, rec.Mindmap.Root, "")
		}
	})
}

// mindmapAction generates or regenerates the mind map from the player bar,
// disabled until there is a transcript. It steps aside while generation runs.
func (a *app) mindmapAction(c *ui.Context, rec store.Recording) {
	a.mu.Lock()
	busy := a.mindmapping[rec.ID]
	a.mu.Unlock()
	if busy {
		return
	}
	label := "生成脑图"
	if rec.Mindmap != nil {
		label = "重新生成"
	}
	if actionButton(c, "gen-mind", label, rec.Transcript == nil).Clicked() {
		a.generateMindmap(rec)
	}
}

// mindNode renders one outline node and, when expanded, its children indented
// below. path is a stable key for the per-node collapse state.
func (a *app) mindNode(c *ui.Context, n *store.MindNode, path string) {
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
