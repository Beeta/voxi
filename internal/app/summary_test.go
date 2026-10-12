package app

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
)

// TestSummaryHeaderClicks covers the summary tab's own row, where the
// generate button and the prompt card below it are two clicks that must not
// be confused for one another: the header toggles the prompt, the button
// generates, and neither may do the other's job. They used to share a row.
//
// The tab is rendered on its own rather than through the app: the player at
// the bottom of the pane reports a missing audio file into the same status
// field every frame, which would drown the evidence that the button ran.
//
// The click on the button runs into the missing-LLM path, which stops before
// any request is made, so the status is that evidence.
func TestSummaryHeaderClicks(t *testing.T) {
	dir := t.TempDir()
	lib, err := store.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rec := lib.Add("会议", filepath.Join(dir, "m.m4a"), 60)
	lib.SetTranscript(rec.ID, &store.Transcript{
		Segments: []store.Segment{{Start: 0, End: 3, Text: "你好"}},
	})
	// Add returned a copy from before the transcript was stored, and the
	// tab reads what it is handed: without this the recording still looks
	// untranscribed and the button stays disabled.
	rec, _ = lib.Get(rec.ID)
	set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &Settings{}}
	a := newApp(lib, set, audio.NewRecorder())

	tt := ui.NewTester(func(c *ui.Context) { a.summaryView(c, rec) }, 968, 704)
	tt.Frame()

	// The prompt starts collapsed; its header expands it, and expanding it
	// is not generating.
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
		t.Errorf("expanding the prompt set the status to %q", a.status)
	}

	// The button generates, and leaves the prompt as it found it.
	if err := tt.Click("生成摘要"); err != nil {
		t.Fatal(err)
	}
	if a.status != "请先在设置里配置大模型" {
		t.Errorf("clicking 生成摘要 left the status %q, want the missing-LLM notice", a.status)
	}
	if !a.promptOpen[rec.ID] {
		t.Error("clicking 生成摘要 collapsed the prompt")
	}
}
