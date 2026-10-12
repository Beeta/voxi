package app

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
)

// summaryApp is an app over a transcribed recording under t.TempDir(), for the
// summary-tab tests. hasTranscript lets a test ask for one that isn't
// transcribed yet, to exercise the disabled action.
func summaryApp(t *testing.T, hasTranscript bool) (*app, store.Recording) {
	t.Helper()
	dir := t.TempDir()
	lib, err := store.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rec := lib.Add("会议", filepath.Join(dir, "m.m4a"), 60)
	if hasTranscript {
		lib.SetTranscript(rec.ID, &store.Transcript{
			Segments: []store.Segment{{Start: 0, End: 3, Text: "你好"}},
		})
	}
	// Add returned a copy from before the transcript was stored; re-read it so
	// the view sees the transcript.
	rec, _ = lib.Get(rec.ID)
	set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &Settings{}}
	return newApp(lib, set, audio.NewRecorder()), rec
}

// TestSummaryPromptToggles covers the one interactive thing left in the
// summary tab body: clicking the prompt header expands and collapses it, and
// nothing else — the generate button moved to the player bar.
func TestSummaryPromptToggles(t *testing.T) {
	a, rec := summaryApp(t, true)
	tt := ui.NewTester(func(c *ui.Context) { a.summaryView(c, rec) }, 968, 704)
	tt.Frame()

	if a.promptOpen[rec.ID] {
		t.Fatal("the prompt started expanded")
	}
	if err := tt.Click("生成指令（可编辑）"); err != nil {
		t.Fatal(err)
	}
	if !a.promptOpen[rec.ID] {
		t.Error("clicking the prompt header did not expand the prompt")
	}
	if a.status != "" {
		t.Errorf("toggling the prompt set the status to %q", a.status)
	}
}

// TestSummaryActionGenerates drives the player-bar action: it fires when there
// is a transcript (running into the missing-LLM path, which sets the status
// before any request), and stays disabled — no status, no click — without one.
// It is rendered on its own, not through the app, so the player's own
// missing-audio status does not drown the evidence.
func TestSummaryActionGenerates(t *testing.T) {
	a, rec := summaryApp(t, true)
	tt := ui.NewTester(func(c *ui.Context) { a.summaryAction(c, rec) }, 400, 80)
	tt.Frame()
	if err := tt.Click("生成摘要"); err != nil {
		t.Fatal(err)
	}
	if a.status != "请先在设置里配置大模型" {
		t.Errorf("clicking 生成摘要 left the status %q, want the missing-LLM notice", a.status)
	}

	b, rbNo := summaryApp(t, false)
	tt2 := ui.NewTester(func(c *ui.Context) { b.summaryAction(c, rbNo) }, 400, 80)
	tt2.Frame()
	if err := tt2.Click("生成摘要"); err != nil {
		t.Fatal(err)
	}
	if b.status != "" {
		t.Errorf("the disabled action fired without a transcript, status %q", b.status)
	}
}
