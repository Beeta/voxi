package app

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/store"
)

// TestQAPresetsDefaultOnce: a settings file from before presets gets the
// built-in ones, but a list the user emptied stays empty.
func TestQAPresetsDefaultOnce(t *testing.T) {
	var old Settings
	json.Unmarshal([]byte(`{"mic":true}`), &old)
	upgradeSettings(&old)
	if len(old.QAPresets) != len(defaultQAPresets) {
		t.Errorf("an old file got %d presets, want the %d built-in", len(old.QAPresets), len(defaultQAPresets))
	}
	old.QAPresets[0].Label = "改过"
	if defaultQAPresets[0].Label == "改过" {
		t.Error("editing a preset changed the built-in list")
	}

	var emptied Settings
	json.Unmarshal([]byte(`{"qaPresets":[]}`), &emptied)
	upgradeSettings(&emptied)
	if len(emptied.QAPresets) != 0 {
		t.Errorf("an emptied list came back with %d presets", len(emptied.QAPresets))
	}
}

// TestQAPresetsSkipEmpty: a preset still being written (no text) shows no
// chip, and one without a label takes the start of its text.
func TestQAPresetsSkipEmpty(t *testing.T) {
	got := qaPresets(Settings{QAPresets: []QAPreset{
		{Label: "空", Text: "  "},
		{Text: "这是一个很长的问题文本"},
	}})
	if len(got) != 1 || got[0].Label != "这是一个很长的问…" {
		t.Errorf("got %+v", got)
	}
}

// qaInputApp renders the Q&A tab of a transcribed recording with chat.
func qaInputApp(t *testing.T, chat ...store.ChatMessage) (*app, *ui.Tester, string) {
	t.Helper()
	a, set := newSettingsApp(t)
	set.update(func(s *Settings) {
		s.LLMProviders = []LLMProvider{{ID: "p", Name: "测试", BaseURL: "http://127.0.0.1:1", Key: "k", Models: []string{"m"}}}
		s.QAPresets = []QAPreset{{Label: "重点", Text: "总结重点"}}
	})
	rec := a.lib.Add("预设测试", filepath.Join(t.TempDir(), "x.m4a"), 1)
	t.Cleanup(func() { a.lib.Remove(rec.ID) })
	a.lib.SetTranscript(rec.ID, &store.Transcript{Segments: []store.Segment{{Text: "内容"}}})
	for _, m := range chat {
		a.lib.AppendChat(rec.ID, m)
	}
	tt := ui.NewTester(func(c *ui.Context) { r, _ := a.lib.Get(rec.ID); a.qaView(c, r) }, 900, 700)
	tt.Frame()
	return a, tt, rec.ID
}

// TestPresetFillsTheInput: clicking a chip puts its text in the input,
// focused with the caret at the end, so typing appends to it.
func TestPresetFillsTheInput(t *testing.T) {
	a, tt, id := qaInputApp(t, store.ChatMessage{Role: "user", Text: "旧"}, store.ChatMessage{Role: "assistant", Text: "答"})
	if err := tt.Click("重点"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	tt.Type("，越短越好")
	tt.Frame()
	if got := a.draftQ[id]; got != "总结重点，越短越好" {
		t.Errorf("the input holds %q", got)
	}
}

// TestEnterSendsShiftEnterBreaks: Enter sends the question and clears the
// box, Shift+Enter is a new line, and Enter in an empty box does nothing at
// all — no send, no blank line.
func TestEnterSendsShiftEnterBreaks(t *testing.T) {
	a, tt, id := qaInputApp(t)
	if err := tt.Click("问题"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.draftQ[id] != "" {
		t.Errorf("Enter in an empty box left %q", a.draftQ[id])
	}
	if r, _ := a.lib.Get(id); len(r.Chat) != 0 {
		t.Fatal("Enter in an empty box sent something")
	}
	tt.Type("第一行")
	tt.Key(ui.Shift, ui.KeyEnter)
	tt.Type("第二行")
	tt.Frame()
	if got := a.draftQ[id]; got != "第一行\n第二行" {
		t.Fatalf("after Shift+Enter the box holds %q", got)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	r, _ := a.lib.Get(id)
	if len(r.Chat) == 0 || r.Chat[0].Text != "第一行\n第二行" {
		t.Fatalf("Enter did not send it: %+v", r.Chat)
	}
	if a.draftQ[id] != "" {
		t.Errorf("the box still holds %q after sending", a.draftQ[id])
	}
	for i := 0; i < 300 && a.isAsking(id); i++ {
		tt.Frame()
	}
}

// TestUpRecallsTheLastQuestion: ↑ in an empty box brings the last question
// back with the caret at its end; in a box with text it is left to the text.
func TestUpRecallsTheLastQuestion(t *testing.T) {
	a, tt, id := qaInputApp(t,
		store.ChatMessage{Role: "user", Text: "第一个"}, store.ChatMessage{Role: "assistant", Text: "答"},
		store.ChatMessage{Role: store.ChatDivider},
		store.ChatMessage{Role: "user", Text: "最后一个"}, store.ChatMessage{Role: "assistant", Text: "答"})
	if err := tt.Click("问题"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	tt.Type("？")
	tt.Frame()
	if got := a.draftQ[id]; got != "最后一个？" {
		t.Errorf("↑ then typing gave %q", got)
	}
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if got := a.draftQ[id]; !strings.HasPrefix(got, "最后一个") {
		t.Errorf("↑ in a box with text replaced it: %q", got)
	}
}

// TestPresetSettingsEdit drives the settings rows: delete the first, and
// the one below must show its own text in its place, not the deleted one's.
func TestPresetSettingsEdit(t *testing.T) {
	a, set := newSettingsApp(t)
	set.update(func(s *Settings) {
		s.QAPresets = []QAPreset{{Label: "甲", Text: "甲的问题"}, {Label: "乙", Text: "乙的问题"}}
	})
	tt := ui.NewTester(func(c *ui.Context) { a.qaPresetSettings(c, set.snapshot()) }, 616, 900)
	tt.Frame()
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if got := set.snapshot().QAPresets; len(got) != 1 || got[0].Label != "乙" {
		t.Fatalf("after deleting the first: %+v", got)
	}
	// The row now at position 0 is 乙's: typing into its label must edit
	// 乙, starting from 乙's text rather than what the deleted row held.
	if err := tt.Click("按钮文字"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(ui.Cmd, ui.KeyRight)
	tt.Type("2")
	tt.Frame()
	if got := set.snapshot().QAPresets[0].Label; got != "乙2" {
		t.Errorf("typing into the remaining row made its label %q, want 乙2", got)
	}
	if err := tt.Click("+ 添加预设"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if n := len(set.snapshot().QAPresets); n != 2 {
		t.Errorf("after 添加 there are %d presets", n)
	}
}
