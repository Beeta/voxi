package app

import (
	"strings"

	"github.com/Beeta/voxi/internal/store"
)

// QAPreset is a saved question for the Q&A tab: Label is what its chip
// shows, Text what it puts in the input. Presets are global — the same
// questions fit every recording.
type QAPreset struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// defaultQAPresets are the presets of a settings file that has never saved
// any. They are copied in on first load, so the user edits their own list.
var defaultQAPresets = []QAPreset{
	{Label: "三个重点", Text: "帮我总结这段内容的三个重点。"},
	{Label: "待办与负责人", Text: "列出所有待办事项，注明负责人和截止时间；没有提到的标「未明确」。"},
	{Label: "未决问题", Text: "还有哪些问题没有定论、需要后续跟进？"},
	{Label: "各方观点", Text: "按说话人分别整理各自的主要观点。"},
}

// usable reports whether the preset has a question to put in the input.
func (p QAPreset) usable() bool { return strings.TrimSpace(p.Text) != "" }

// label is what the preset's chip shows: its label, or the start of its
// question when it has none.
func (p QAPreset) label() string {
	if l := strings.TrimSpace(p.Label); l != "" {
		return l
	}
	return cutRunes(strings.TrimSpace(p.Text), 8)
}

// cutRunes is s cut to n runes, with an ellipsis when it was longer.
func cutRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// lastQuestion is the latest question asked in chat, for ↑ to bring back.
func lastQuestion(chat []store.ChatMessage) string {
	for i := len(chat) - 1; i >= 0; i-- {
		if chat[i].Role == "user" && strings.TrimSpace(chat[i].Text) != "" {
			return chat[i].Text
		}
	}
	return ""
}
