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

// TestPresetLabel: a preset with no question is not usable, and one without
// a label shows the start of its question.
func TestPresetLabel(t *testing.T) {
	if (QAPreset{Label: "空", Text: "  "}).usable() {
		t.Error("a preset without a question is usable")
	}
	if got := (QAPreset{Text: "这是一个很长的问题文本"}).label(); got != "这是一个很长的问…" {
		t.Errorf("label %q", got)
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

// TestPresetRightClickEditAndDelete drives the chips' context menu on the
// Q&A tab: 编辑 opens the dialog on that preset and 保存 writes the draft;
// 取消 leaves it as it was; 删除 asks, then removes that one and no other.
func TestPresetRightClickEditAndDelete(t *testing.T) {
	a, tt, _ := qaInputApp(t, store.ChatMessage{Role: "user", Text: "旧"}, store.ChatMessage{Role: "assistant", Text: "答"})
	a.settings.update(func(s *Settings) {
		s.QAPresets = []QAPreset{{Label: "甲", Text: "甲的问题"}, {Label: "乙", Text: "乙的问题"}}
	})
	tt.Frame()
	if err := tt.RightClick("乙"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("预设问题"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("乙的新问题")
	tt.Frame()
	if got := a.settings.snapshot().QAPresets[1].Text; got != "乙的问题" {
		t.Fatalf("the preset changed before 保存: %q", got)
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.settings.snapshot().QAPresets; got[1].Text != "乙的新问题" || got[0].Text != "甲的问题" {
		t.Fatalf("after 保存: %+v", got)
	}

	tt.RightClick("甲")
	tt.ChooseMenuItem("编辑")
	tt.Frame()
	tt.Click("预设问题")
	tt.Type("不要")
	tt.Click("取消")
	tt.Frame()
	if got := a.settings.snapshot().QAPresets[0].Text; got != "甲的问题" {
		t.Errorf("取消 still saved: %q", got)
	}

	if err := tt.RightClick("甲"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("删除预设？") {
		t.Fatal("删除 did not ask first")
	}
	if err := tt.Click("删除预设"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.settings.snapshot().QAPresets; len(got) != 1 || got[0].Label != "乙" {
		t.Errorf("after deleting 甲: %+v", got)
	}
}

// TestPresetAdd: + opens an empty editor, 保存 stays off until there is a
// question, and the new preset shows as a chip.
func TestPresetAdd(t *testing.T) {
	a, tt, _ := qaInputApp(t, store.ChatMessage{Role: "user", Text: "旧"}, store.ChatMessage{Role: "assistant", Text: "答"})
	if err := tt.Click("+"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Click("保存")
	tt.Frame()
	if n := len(a.settings.snapshot().QAPresets); n != 1 {
		t.Fatalf("an empty preset was saved: %d presets", n)
	}
	tt.Click("按钮文字")
	tt.Type("新的")
	tt.Click("预设问题")
	tt.Type("新的问题")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.settings.snapshot().QAPresets; len(got) != 2 || got[1] != (QAPreset{Label: "新的", Text: "新的问题"}) {
		t.Fatalf("after adding: %+v", got)
	}
	if !tt.HasText("新的") {
		t.Error("the new preset has no chip")
	}
}

// TestMorePresetsCanBeManaged: presets past the chip limit sit in 更多, and
// each is still usable, editable and deletable from its submenu.
func TestMorePresetsCanBeManaged(t *testing.T) {
	a, tt, id := qaInputApp(t, store.ChatMessage{Role: "user", Text: "旧"}, store.ChatMessage{Role: "assistant", Text: "答"})
	a.settings.update(func(s *Settings) {
		s.QAPresets = nil
		for _, l := range []string{"一", "二", "三", "四", "五", "六"} {
			s.QAPresets = append(s.QAPresets, QAPreset{Label: l, Text: l + "的问题"})
		}
	})
	tt.Frame()
	if tt.HasText("六") {
		t.Fatal("the sixth preset shows as a chip")
	}
	if err := tt.Click("更多"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("六", "使用"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.draftQ[id]; got != "六的问题" {
		t.Errorf("使用 put %q in the input", got)
	}
	tt.Click("更多")
	if err := tt.ChooseMenuItem("六", "删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Click("删除预设")
	tt.Frame()
	if n := len(a.settings.snapshot().QAPresets); n != 5 {
		t.Errorf("after deleting 六 there are %d presets", n)
	}
}
