package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/store"
)

// qaView is the Q&A tab: a scrollable chat log over a fixed input row.
func (a *app) qaView(c *ui.Context, rec store.Recording) {
	a.mu.Lock()
	busy := a.asking[rec.ID]
	a.mu.Unlock()
	ui.Column(c).Grow(1).Children(func() {
		ui.Scroll(c.Key("qa-" + rec.ID)).Grow(1).Children(func() {
			ui.Column(c).Padding(20, 32).Gap(12).Children(func() {
				if len(rec.Chat) == 0 {
					ui.Text(c, "就这段录音的内容提问，例如「帮我总结三个重点」。").FontSize(13).TextColor(muted)
				}
				for i := range rec.Chat {
					a.chatBubble(c, i, rec.Chat[i])
				}
			})
		})
		a.qaInput(c, rec, busy)
	})
}

// chatBubble renders one Q&A turn as a left/right aligned bubble.
func (a *app) chatBubble(c *ui.Context, i int, m store.ChatMessage) {
	user := m.Role == "user"
	text := m.Text
	if text == "" {
		text = "…"
	}
	ui.Row(c.Key(i)).Children(func() {
		if user {
			ui.Spacer(c)
		}
		bg, fg := cardBg, ink
		if user {
			bg, fg = accent, onAccent
		}
		ui.Column(c).Padding(10, 16).Radius(10).Background(bg).MaxWidth(460).Children(func() {
			ui.Text(c, text).FontSize(14).TextColor(fg).Selectable()
		})
		if !user {
			ui.Spacer(c)
		}
	})
}

// qaInput is the bottom input: a growing text box plus a send button.
func (a *app) qaInput(c *ui.Context, rec store.Recording, busy bool) {
	hasTranscript := rec.Transcript != nil
	ui.Column(c).Children(func() {
		ui.Column(c).Height(1).Background(line)
		ui.Row(c).Padding(12, 32).Gap(10).AlignItems(ui.End).Children(func() {
			q := a.draftQ[rec.ID]
			ta := ui.TextArea(c.Key("qa-input-"+rec.ID), &q).Lines(1, 4).Grow(1).FontSize(14).TextColor(ink)
			ta.Placeholder("输入问题…")
			if ta.Changed() {
				a.draftQ[rec.ID] = q
			}
			a.quickModelSelect(c, "qa-model")
			off := busy || !hasTranscript || strings.TrimSpace(a.draftQ[rec.ID]) == ""
			b := ui.ButtonBase(c.Key("send")).Padding(10, 16).Radius(8).Center().Background(accent).Disabled(off)
			if off {
				b.Background(cardBg2)
			}
			b.Children(func() {
				fg := onAccent
				if off {
					fg = muted
				}
				ui.Text(c, "发送").FontSize(13).Bold().TextColor(fg)
			})
			if !off && b.Clicked() {
				a.ask(rec, a.draftQ[rec.ID])
				a.draftQ[rec.ID] = ""
			}
		})
	})
}
