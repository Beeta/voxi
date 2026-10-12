package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/llm"
	"github.com/Beeta/voxi/internal/md"
	"github.com/Beeta/voxi/internal/store"
)

// mdPalette is how a summary's Markdown is drawn, in the app's palette.
var mdPalette = md.Palette{
	Text:    ink,
	Muted:   muted,
	Accent:  accent,
	Surface: cardBg2,
	Border:  line,
	Size:    14,
}

// mdEntry is a parsed Markdown document, kept so the view does not reparse
// it on every frame.
type mdEntry struct {
	src    string
	blocks []md.Block
}

// summaryView is the summary tab: an editable generation prompt on top, then
// the generated summary (overview + highlights + action items) below.
func (a *app) summaryView(c *ui.Context, rec store.Recording) {
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
func (a *app) summaryPrompt(c *ui.Context, rec store.Recording, busy bool) {
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
					cur = llm.DefaultSummaryPrompt
				}
			}
			ta := ui.TextArea(c.Key("prompt-"+rec.ID), &cur).Lines(2, 5).FontSize(13).TextColor(ink)
			if ta.Changed() {
				a.draftPrompt[rec.ID] = cur
			}
			ui.Text(c, "指令会原样发给模型，不加任何东西。返回的 JSON 含 overview / highlights / actions 时下面分块显示，其他格式整段显示。").
				FontSize(11).TextColor(muted)
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
					a.lib.SetSummaryPrompt(rec.ID, d)
					rec.SummaryPrompt = d
				}
				a.generateSummary(rec)
			}
		})
	})
}

// summaryBody renders a generated summary. A reply that parsed as the
// summary's shape has three parts and is drawn in three sections; anything
// else landed whole in Overview, which is drawn as the Markdown document it
// most likely is.
func (a *app) summaryBody(c *ui.Context, rec store.Recording) {
	sm := rec.Summary
	ui.Column(c).Gap(16).Children(func() {
		if sm.Stale {
			ui.Text(c, "⚠ 转写已更新，摘要可能过期，建议重新生成。").FontSize(12).TextColor(recRed)
		}
		if len(sm.Highlights) == 0 && len(sm.Actions) == 0 {
			a.markdown(c, rec.ID+":ov", sm.Overview)
			return
		}
		section(c, "概览", func() { a.markdown(c, rec.ID+":ov", sm.Overview) })
		if len(sm.Highlights) > 0 {
			section(c, "关键要点", func() { a.markdown(c, rec.ID+":hl", bulletSource(sm.Highlights)) })
		}
		if len(sm.Actions) > 0 {
			section(c, "待办事项", func() { a.markdown(c, rec.ID+":ac", bulletSource(sm.Actions)) })
		}
	})
}

// bulletSource writes items as a Markdown list, so they come out with the
// same bullets and inline styles as the rest of the document. Continuation
// lines are indented to stay part of their item.
func bulletSource(items []string) string {
	var b strings.Builder
	for _, it := range items {
		for i, line := range strings.Split(strings.TrimSpace(it), "\n") {
			if i == 0 {
				b.WriteString("- ")
			} else {
				b.WriteString("  ")
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// markdown draws src as Markdown. The view runs every frame, so the parse is
// cached under key and redone only when src changes.
func (a *app) markdown(c *ui.Context, key, src string) {
	e, ok := a.mdCache[key]
	if !ok || e.src != src {
		e = mdEntry{src: src, blocks: md.Parse(src)}
		a.mdCache[key] = e
	}
	md.Render(c, e.blocks, mdPalette)
}
