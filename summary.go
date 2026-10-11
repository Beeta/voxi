package main

import "github.com/egoist/mygo/ui"

// summaryView is the summary tab: an editable generation prompt on top, then
// the generated summary (overview + highlights + action items) below.
func (a *app) summaryView(c *ui.Context, rec Recording) {
	a.mu.Lock()
	busy := a.summarizing[rec.ID]
	a.mu.Unlock()
	ui.Column(c).Gap(16).Children(func() {
		a.summaryPrompt(c, rec, busy)
		switch {
		case busy:
			loadingRow(c, "正在生成摘要…")
		case rec.Summary == nil:
			ui.Text(c, "还没有摘要。编辑上面的生成指令后点击「生成摘要」。").FontSize(13).TextColor(muted)
		default:
			a.summaryBody(c, rec)
		}
	})
}

// summaryPrompt shows the generate button and a collapsible, editable
// instruction (collapsed by default; click the header to edit it).
func (a *app) summaryPrompt(c *ui.Context, rec Recording, busy bool) {
	hasTranscript := rec.Transcript != nil
	open := a.promptOpen[rec.ID] // zero value = collapsed
	ui.Column(c).Gap(10).Padding(14).Radius(10).Background(cardBg).Children(func() {
		head := ui.Row(c).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		head.Children(func() {
			ui.Text(c, triangle(open)).FontSize(12).TextColor(muted).Width(14)
			ui.Text(c, "生成指令（可编辑）").FontSize(12).Bold().TextColor(muted).Grow(1)
		})
		if head.Clicked() {
			a.promptOpen[rec.ID] = !open
		}
		if open {
			cur, ok := a.draftPrompt[rec.ID]
			if !ok {
				if cur = rec.SummaryPrompt; cur == "" {
					cur = defaultSummaryPrompt
				}
			}
			ta := ui.TextArea(c.Key("prompt-"+rec.ID), &cur).Lines(2, 5).FontSize(13).TextColor(ink)
			if ta.Changed() {
				a.draftPrompt[rec.ID] = cur
			}
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if !hasTranscript {
				ui.Text(c, "请先在「转写」页签完成识别。").FontSize(12).TextColor(muted)
			}
			ui.Spacer(c)
			label := "生成摘要"
			if rec.Summary != nil {
				label = "重新生成"
			}
			off := busy || !hasTranscript
			b := ui.ButtonBase(c.Key("gen-sum")).Padding(8, 14).Radius(8).Center().Background(accent).Disabled(off)
			if off {
				b.Background(cardBg2)
			}
			b.Children(func() {
				fg := ui.Hex("#ffffff")
				if off {
					fg = muted
				}
				ui.Text(c, label).FontSize(12).Bold().TextColor(fg)
			})
			if !off && b.Clicked() {
				if d, ok := a.draftPrompt[rec.ID]; ok {
					a.store.setSummaryPrompt(rec.ID, d)
					rec.SummaryPrompt = d
				}
				a.generateSummary(rec)
			}
		})
	})
}

// summaryBody renders a generated summary.
func (a *app) summaryBody(c *ui.Context, rec Recording) {
	sm := rec.Summary
	ui.Column(c).Gap(16).Children(func() {
		if sm.Stale {
			ui.Text(c, "⚠ 转写已更新，摘要可能过期，建议重新生成。").FontSize(12).TextColor(recRed)
		}
		section(c, "概览", func() {
			ui.Text(c, sm.Overview).FontSize(14).TextColor(ink).Selectable()
		})
		if len(sm.Highlights) > 0 {
			section(c, "关键要点", func() { bulletList(c, sm.Highlights) })
		}
		if len(sm.Actions) > 0 {
			section(c, "待办事项", func() { bulletList(c, sm.Actions) })
		}
	})
}
