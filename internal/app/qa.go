package app

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Beeta/voxi/internal/store"
	"github.com/egoist/mygo/ui"
)

// qaView is the Q&A tab: a toolbar, the scrollable chat log, and the input
// row. With the history open it shows the archived conversations instead.
func (a *app) qaView(c *ui.Context, rec store.Recording) {
	a.mu.Lock()
	busy := a.asking[rec.ID]
	a.mu.Unlock()
	ui.Column(c).Grow(1).Children(func() {
		a.qaToolbar(c, rec, busy)
		if a.qaHistory {
			a.qaArchive(c, rec, busy)
		} else {
			ui.Scroll(c.Key("qa-" + rec.ID)).Grow(1).Children(func() {
				ui.Column(c).Padding(20, 32).Gap(12).Children(func() {
					if len(rec.Chat) == 0 {
						a.qaEmpty(c, rec)
					}
					last := len(rec.Chat) - 1
					for i := range rec.Chat {
						a.chatTurn(c, rec, i, i == last, busy)
					}
				})
			})
			a.qaInput(c, rec, busy)
		}
	})
	a.presetDialogs(c)
	if i := ui.AlertDialog(c.Key("qa-clear"), &a.confirmClearChat, "清空当前对话？",
		"将删除当前对话的全部问答，不会进入历史会话，无法恢复。", "清空", "取消"); i == 0 {
		a.lib.ClearChat(rec.ID)
	}
}

// qaToolbar is the row over the chat: 新话题 (cut the context), 归档,
// 清空, and the way into and out of the archived conversations.
func (a *app) qaToolbar(c *ui.Context, rec store.Recording, busy bool) {
	empty := len(rec.Chat) == 0
	endsWithDivider := !empty && rec.Chat[len(rec.Chat)-1].Role == store.ChatDivider
	ui.Column(c).Children(func() {
		ui.Row(c).Padding(8, 32).Gap(8).AlignItems(ui.Center).Children(func() {
			if a.qaHistory {
				if toolButton(c, "qa-back", "← 返回当前对话", false).Clicked() {
					a.qaHistory = false
				}
				ui.Spacer(c)
				return
			}
			if toolButton(c, "qa-topic", "新话题", busy || empty || endsWithDivider).
				Tooltip("之前的问答保留在屏幕上，但不再发给模型").Clicked() {
				a.lib.NewChatTopic(rec.ID)
			}
			if toolButton(c, "qa-archive", "归档", busy || empty).
				Tooltip("把当前对话存入历史会话，开始一个空白对话").Clicked() {
				a.lib.ArchiveChat(rec.ID)
			}
			if toolButton(c, "qa-clear", "清空", busy || empty).Clicked() {
				a.confirmClearChat = true
			}
			ui.Spacer(c)
			label := "历史会话"
			if n := len(rec.ChatArchive); n > 0 {
				label += "（" + strconv.Itoa(n) + "）"
			}
			if toolButton(c, "qa-history", label, len(rec.ChatArchive) == 0).Clicked() {
				a.qaHistory = true
			}
		})
		ui.Column(c).Height(1).Background(line)
	})
}

// toolButton is a small outlined button for the Q&A toolbar and the actions
// under an answer.
func toolButton(c *ui.Context, key, label string, off bool) ui.Element {
	fg := ink
	if off {
		fg = muted
	}
	b := ui.ButtonBase(c.Key(key)).Padding(4, 10).Radius(6).Center().Border(1, line).Disabled(off)
	b.Background(controlFill(b, cardBg2, cardBg3))
	b.Children(func() { ui.Text(c, label).FontSize(12).TextColor(fg) })
	return b
}

// chatTurn renders one message: a question, an answer (as Markdown) with its
// actions under it, or a 新话题 divider.
func (a *app) chatTurn(c *ui.Context, rec store.Recording, i int, last, busy bool) {
	m := rec.Chat[i]
	if m.Role == store.ChatDivider {
		ui.Row(c.Key(i)).Gap(10).AlignItems(ui.Center).Padding(4, 0).Children(func() {
			ui.Column(c).Height(1).Grow(1).Background(line)
			ui.Text(c, "新话题 · 以上内容不再作为上下文").FontSize(11).TextColor(muted)
			ui.Column(c).Height(1).Grow(1).Background(line)
		})
		return
	}
	streaming := busy && last && m.Role == "assistant"
	ui.Column(c.Key(i)).Gap(4).Children(func() {
		a.chatBubble(c, rec.ID, i, m, streaming)
		if streaming || busy {
			if streaming {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c)
					if toolButton(c, "qa-stop", "停止生成", false).Clicked() {
						a.stopAnswer(rec.ID)
					}
				})
			}
			return
		}
		ui.Row(c).Gap(6).Children(func() {
			if m.Role == "user" {
				ui.Spacer(c)
			}
			if m.Role == "assistant" && !m.Error && toolButton(c, "qa-copy", "复制", false).Clicked() {
				c.Services().WriteClipboard(m.Text)
				a.setStatus("已复制回答")
			}
			if m.Role == "assistant" && last && toolButton(c, "qa-regen", "重新生成", false).Clicked() {
				a.regenerate(rec)
			}
			if toolButton(c, "qa-del", "删除", false).Tooltip("删除这一问一答").Clicked() {
				a.lib.DeleteChat(rec.ID, i)
			}
		})
	})
}

// chatBubble draws one question or answer. Answers are Markdown — the model
// writes lists, tables and code — and questions are the plain text typed.
func (a *app) chatBubble(c *ui.Context, recID string, i int, m store.ChatMessage, streaming bool) {
	user := m.Role == "user"
	ui.Row(c).Children(func() {
		if user {
			ui.Spacer(c)
		}
		bg := cardBg
		if user {
			bg = accent
		}
		ui.Column(c).Padding(10, 16).Radius(10).Background(bg).MaxWidth(560).Children(func() {
			switch {
			case user:
				ui.Text(c, m.Text).FontSize(14).TextColor(onAccent).Selectable()
			case m.Text == "" && streaming:
				ui.Text(c, "…").FontSize(14).TextColor(muted)
			case m.Error:
				ui.Text(c, m.Text).FontSize(13).TextColor(recRed).Selectable()
			default:
				a.markdown(c, recID+":qa:"+strconv.Itoa(i), m.Text)
			}
		})
		if !user {
			ui.Spacer(c)
		}
	})
}

// qaArchive lists the archived conversations, newest first, each openable
// for reading, restorable as the current one (not while an answer streams
// into the current one), or deletable.
func (a *app) qaArchive(c *ui.Context, rec store.Recording, busy bool) {
	ui.Scroll(c.Key("qa-arch-" + rec.ID)).Grow(1).Children(func() {
		ui.Column(c).Padding(20, 32).Gap(12).Children(func() {
			if len(rec.ChatArchive) == 0 {
				ui.Text(c, "还没有归档的对话。").FontSize(13).TextColor(muted)
			}
			for _, sess := range rec.ChatArchive {
				a.archivedSession(c, rec, sess, busy)
			}
		})
	})
	if i := ui.AlertDialog(c.Key("qa-delsess"), &a.confirmDelSession, "删除这条历史会话？",
		"删除后无法恢复。", "删除", "取消"); i == 0 {
		a.lib.DeleteChatSession(rec.ID, a.delSessionID)
	}
}

// archivedSession is one archived conversation: a header with its title,
// when it was archived and how many questions it holds, and, expanded, the
// conversation itself, read-only.
func (a *app) archivedSession(c *ui.Context, rec store.Recording, sess store.ChatSession, busy bool) {
	open := a.qaOpenSession == sess.ID
	questions := 0
	for _, m := range sess.Messages {
		if m.Role == "user" {
			questions++
		}
	}
	ui.Column(c.Key(sess.ID)).Gap(10).Padding(12, 14).Radius(10).Border(1, line).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			head := ui.ButtonBase(c.Key("open")).Grow(1).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, triangle(open)).FontSize(12).TextColor(muted).Width(14)
					ui.Column(c).Gap(2).Grow(1).Children(func() {
						ui.Text(c, sess.Title).FontSize(14).TextColor(ink).SingleLine()
						ui.Text(c, sess.Archived.Local().Format("01-02 15:04")+" · "+strconv.Itoa(questions)+" 个问题").FontSize(12).TextColor(muted)
					})
				})
			})
			if head.Clicked() {
				if open {
					a.qaOpenSession = ""
				} else {
					a.qaOpenSession = sess.ID
				}
			}
			if toolButton(c, "restore", "恢复为当前对话", busy).
				Tooltip("当前对话不为空时，会先把它归档").Clicked() {
				a.lib.RestoreChat(rec.ID, sess.ID)
				a.qaHistory = false
			}
			del := ui.ButtonBase(c.Key("del")).Padding(4, 10).Radius(6).Center().Border(1, line)
			del.Children(func() { ui.Text(c, "删除").FontSize(12).TextColor(recRed) })
			if del.Clicked() {
				a.delSessionID, a.confirmDelSession = sess.ID, true
			}
		})
		if !open {
			return
		}
		for i, m := range sess.Messages {
			if m.Role == store.ChatDivider {
				ui.Text(c.Key(i), "—— 新话题 ——").FontSize(11).TextColor(muted)
				continue
			}
			ui.Column(c.Key(i)).Children(func() {
				a.chatBubble(c, sess.ID, i, m, false)
			})
		}
	})
}

// qaInput is the bottom input: the preset chips, then a growing text box,
// the model picker and the send button. Enter sends and Shift+Enter breaks
// the line; ↑ in an empty box brings back the last question.
func (a *app) qaInput(c *ui.Context, rec store.Recording, busy bool) {
	hasTranscript := rec.Transcript != nil
	canSend := func() bool {
		return !busy && hasTranscript && strings.TrimSpace(a.draftQ[rec.ID]) != ""
	}
	send := func() {
		a.ask(rec, a.draftQ[rec.ID])
		a.draftQ[rec.ID] = ""
	}
	ui.Column(c).Children(func() {
		ui.Column(c).Height(1).Background(line)
		// An empty conversation offers the presets as cards above (qaEmpty);
		// chips here as well would say the same thing twice.
		if len(rec.Chat) > 0 {
			a.presetChips(c, rec)
		}
		ui.Row(c).Padding(10, 32, 12, 32).Gap(10).AlignItems(ui.End).Children(func() {
			q := a.draftQ[rec.ID]
			ta := ui.TextArea(c.Key("qa-input-"+rec.ID), &q).Lines(1, 4).Grow(1).FontSize(14).TextColor(ink)
			ta.Label("问题").Placeholder("输入问题…  Enter 发送，Shift+Enter 换行，↑ 调出上一个问题")
			// Keys come here before the text box sees them. While an input
			// method composes, macOS keeps Enter for choosing the candidate
			// and never delivers it, so this cannot send half a word.
			ta.HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind != ui.InputKeyDown || ev.Mods != 0 {
					return false
				}
				switch ev.Key {
				case ui.KeyEnter:
					// Taken even when there is nothing to send, so a
					// stray Enter never leaves a blank line behind.
					if canSend() {
						send()
					}
					return true
				case ui.KeyUp:
					if a.draftQ[rec.ID] != "" {
						return false // ↑ moves the caret in the text
					}
					if last := lastQuestion(rec.Chat); last != "" {
						a.draftQ[rec.ID] = last
						a.qaCaretEnd = true
						return true
					}
				}
				return false
			})
			if a.qaFocusInput {
				ta.Focus()
				a.qaFocusInput = false
			}
			if a.qaCaretEnd {
				n := len([]rune(a.draftQ[rec.ID]))
				ta.SetTextSelection(n, n)
				a.qaCaretEnd = false
			}
			if ta.Changed() {
				a.draftQ[rec.ID] = q
			}
			a.quickModelSelect(c, "qa-model")
			off := !canSend()
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
				send()
			}
		})
	})
}

// presetChips is the row of saved questions over the input. A click puts
// the question in the input, replacing what is there, with the caret at its
// end, for it to be edited and sent; a right click offers 编辑 and 删除.
// The row ends with + to add one. Past qaChipLimit the rest go in a 更多
// menu, each as a submenu of 使用, 编辑 and 删除, since a menu item takes no
// right click.
func (a *app) presetChips(c *ui.Context, rec store.Recording) {
	all := a.settings.snapshot().QAPresets
	ui.Row(c.Key("qa-presets")).Padding(10, 32, 0, 32).Gap(6).AlignItems(ui.Center).Children(func() {
		shown := 0
		var more []int
		for i, p := range all {
			if !p.usable() {
				continue
			}
			if shown == qaChipLimit {
				more = append(more, i)
				continue
			}
			shown++
			chip := ui.ButtonBase(c.Key("qa-chip-"+strconv.Itoa(i))).Padding(4, 10).Radius(999).Border(1, line).Tooltip(p.Text)
			chip.Background(controlFill(chip, cardBg2, cardBg3))
			chip.Children(func() { ui.Text(c, p.label()).FontSize(12).TextColor(ink).SingleLine() })
			if chip.Clicked() {
				a.usePreset(rec.ID, p)
			}
			a.presetMenu(chip, i)
		}
		if len(more) > 0 {
			ui.MenuButton(c.Key("qa-more"), "更多", func(m *ui.Menu) {
				for _, i := range more {
					m.Submenu(all[i].label(), func(sm *ui.Menu) {
						if sm.Item("使用").Chosen() {
							a.usePreset(rec.ID, all[i])
						}
						if sm.Item("编辑").Chosen() {
							a.openPresetEditor(i)
						}
						if sm.Item("删除").Chosen() {
							a.delPresetIdx, a.confirmDelPreset = i, true
						}
					})
				}
			}).FontSize(12)
		}
		add := toolButton(c, "qa-preset-add", "+", false).Tooltip("添加预设问题")
		if add.Clicked() {
			a.openPresetEditor(-1)
		}
	})
}

// presetMenu gives preset chip i its right-click menu.
func (a *app) presetMenu(chip ui.Element, i int) {
	chip.ContextMenu(func(m *ui.Menu) {
		if m.Item("编辑").Chosen() {
			a.openPresetEditor(i)
		}
		if m.Item("删除").Chosen() {
			a.delPresetIdx, a.confirmDelPreset = i, true
		}
	})
}

// usePreset puts p in rec's input, focused with the caret at the end.
func (a *app) usePreset(recID string, p QAPreset) {
	a.draftQ[recID] = p.Text
	a.qaFocusInput, a.qaCaretEnd = true, true
}

// qaChipLimit is how many presets show as chips before the rest go in 更多.
const qaChipLimit = 5

// openPresetEditor opens the preset dialog on preset i, or on a new one
// when i is -1.
func (a *app) openPresetEditor(i int) {
	a.presetEditIdx = i
	a.presetEditLabel, a.presetEditText = "", ""
	if all := a.settings.snapshot().QAPresets; i >= 0 && i < len(all) {
		a.presetEditLabel, a.presetEditText = all[i].Label, all[i].Text
	}
	a.presetEditOpen = true
}

// presetDialogs draws the preset editor and the delete confirmation. The
// editor works on a draft and saves on 保存, so a half-typed preset never
// shows up as a chip, and 取消 really leaves the preset as it was.
func (a *app) presetDialogs(c *ui.Context) {
	if a.presetEditOpen {
		title := "编辑预设"
		if a.presetEditIdx < 0 {
			title = "添加预设"
		}
		ui.Modal(c.Key("qa-preset-edit"), &a.presetEditOpen, func() {
			ui.Column(c).Width(440).Gap(12).Children(func() {
				ui.Text(c, title).FontSize(16).Bold().TextColor(ink)
				ui.Text(c, "按钮文字").FontSize(12).TextColor(muted)
				lt := ui.TextInput(c.Key("qa-preset-label"), &a.presetEditLabel).FontSize(13).TextColor(ink)
				lt.Label("按钮文字").Placeholder("留空则取问题开头")
				lt.Changed()
				ui.Text(c, "问题").FontSize(12).TextColor(muted)
				ta := ui.TextArea(c.Key("qa-preset-text"), &a.presetEditText).Lines(3, 8).FontSize(13).TextColor(ink)
				ta.Label("预设问题").Placeholder("点这个预设时填进输入框的问题")
				ta.Changed()
				ui.Row(c).Gap(8).Children(func() {
					ui.Spacer(c)
					if toolButton(c, "qa-preset-cancel", "取消", false).Clicked() {
						a.presetEditOpen = false
					}
					empty := strings.TrimSpace(a.presetEditText) == ""
					save := ui.ButtonBase(c.Key("qa-preset-save")).Padding(6, 16).Radius(8).Center().Background(accent).Disabled(empty)
					if empty {
						save.Background(cardBg2)
					}
					save.Children(func() {
						fg := onAccent
						if empty {
							fg = muted
						}
						ui.Text(c, "保存").FontSize(13).Bold().TextColor(fg)
					})
					if !empty && save.Clicked() {
						a.savePreset()
						a.presetEditOpen = false
					}
				})
			})
		})
	}
	if i := ui.AlertDialog(c.Key("qa-preset-delq"), &a.confirmDelPreset, "删除预设？",
		"「"+a.presetLabelAt(a.delPresetIdx)+"」将从预设里删除。", "删除预设", "取消"); i == 0 {
		idx := a.delPresetIdx
		a.settings.update(func(st *Settings) {
			if idx >= 0 && idx < len(st.QAPresets) {
				st.QAPresets = slices.Delete(st.QAPresets, idx, idx+1)
			}
		})
	}
}

// savePreset writes the editor's draft back: over the preset it opened on,
// or as a new one at the end.
func (a *app) savePreset() {
	p := QAPreset{Label: strings.TrimSpace(a.presetEditLabel), Text: strings.TrimSpace(a.presetEditText)}
	idx := a.presetEditIdx
	a.settings.update(func(st *Settings) {
		if idx >= 0 && idx < len(st.QAPresets) {
			st.QAPresets[idx] = p
		} else {
			st.QAPresets = append(st.QAPresets, p)
		}
	})
}

// presetLabelAt is preset i's chip label, for the delete confirmation.
func (a *app) presetLabelAt(i int) string {
	if all := a.settings.snapshot().QAPresets; i >= 0 && i < len(all) {
		return all[i].label()
	}
	return ""
}

// qaEmpty is what an empty conversation shows: a hint and, as a start, the
// presets as larger cards with their full text. They take the same right
// click as the chips, and a last card adds one.
func (a *app) qaEmpty(c *ui.Context, rec store.Recording) {
	all := a.settings.snapshot().QAPresets
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, "就这段录音的内容提问，或从下面选一个开始（右键预设可编辑、删除）。").FontSize(13).TextColor(muted)
		ui.Row(c).Wrap().Gap(8).Children(func() {
			for i, p := range all {
				if !p.usable() {
					continue
				}
				card := ui.ButtonBase(c.Key("qa-start-"+strconv.Itoa(i))).Padding(10, 14).Radius(10).Border(1, line).Width(260)
				card.Background(controlFill(card, cardBg, cardBg2))
				card.Children(func() {
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, p.label()).FontSize(13).Bold().TextColor(ink)
						ui.Text(c, p.Text).FontSize(12).TextColor(muted).MaxLines(2)
					})
				})
				if card.Clicked() {
					a.usePreset(rec.ID, p)
				}
				a.presetMenu(card, i)
			}
			add := ui.ButtonBase(c.Key("qa-start-add")).Padding(10, 14).Radius(10).Border(1, line).Width(260).Center()
			add.Background(controlFill(add, cardBg, cardBg2))
			add.Children(func() { ui.Text(c, "+ 添加预设").FontSize(13).TextColor(muted) })
			if add.Clicked() {
				a.openPresetEditor(-1)
			}
		})
	})
}
