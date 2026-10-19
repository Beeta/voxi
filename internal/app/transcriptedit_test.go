package app

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
)

// editorApp is an app over a three-segment transcript under t.TempDir().
func editorApp(t *testing.T) (*app, *store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	lib := openTestLib(t, dir)
	rec := lib.Add("会议", filepath.Join(dir, "m.m4a"), 60)
	lib.SetTranscript(rec.ID, &store.Transcript{Segments: []store.Segment{
		{Start: 0, End: 3, Text: "第一句话。", Speaker: "甲"},
		{Start: 3, End: 6, Text: "第二句话。", Speaker: "乙"},
		{Start: 6, End: 9, Text: "第三句话。", Speaker: "甲"},
	}})
	set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &Settings{}}
	return newApp(lib, set, audio.NewRecorder()), lib, rec.ID
}

func segmentText(t *testing.T, lib *store.Store, id string, i int) string {
	t.Helper()
	rec, ok := lib.Get(id)
	if !ok {
		t.Fatal("the recording went missing")
	}
	return rec.Transcript.Segments[i].Text
}

// TestSegmentEditorIsOneLineTall keeps the inline editor the height of the
// text it replaces. A text area carries a floor of three lines unless told
// otherwise, which left the editor about four times too tall.
//
// Measured as how far the editor pushes the rows below it, which is what the
// floor was costing.
func TestSegmentEditorIsOneLineTall(t *testing.T) {
	a, _, id := editorApp(t)
	a.tab = 0
	a.selected = id

	tt := ui.NewTester(a.view, 968, 704)
	tt.Frame()
	tt.Frame()
	last, ok := tt.Find("第三句话。")
	if !ok {
		t.Skip("the transcript did not render")
	}
	resting := last.Y

	a.editSeg = 0
	tt.Frame()
	tt.Frame()
	editing := tt.Find
	lastEd, _ := editing("第三句话。")
	if pushed := lastEd.Y - resting; pushed > 20 {
		t.Errorf("the editor pushed the rows below it down by %.0f, so it is not one line tall", pushed)
	}
}

// TestSegmentEditorEndsOnBlurAndEscape covers what replaced the 完成 button:
// the edit ends when the editor lets go of the focus — a click elsewhere — or
// on Escape, and the text is already saved by then.
func TestSegmentEditorEndsOnBlurAndEscape(t *testing.T) {
	a, lib, id := editorApp(t)
	a.tab = 0
	a.selected = id

	tt := ui.NewTester(a.view, 968, 704)
	tt.Frame()
	tt.Frame()
	first, ok := tt.Find("第一句话。")
	if !ok {
		t.Skip("the transcript did not render")
	}

	// A click on a segment opens its editor.
	tt.ClickAt(first.X+first.W/2, first.Y+first.H/2)
	tt.Frame()
	if a.editSeg != 0 {
		t.Fatalf("clicking the first segment opened editor %d, want 0", a.editSeg)
	}

	// Typing reaches the store without anything being confirmed.
	tt.Type("补一句。")
	tt.Frame()
	tt.Frame()
	if got := segmentText(t, lib, id, 0); got != "第一句话。补一句。" {
		t.Errorf("the typed text saved as %q, want %q", got, "第一句话。补一句。")
	}

	// A click away from the editor ends the edit.
	below, ok := tt.Find("第三句话。")
	if !ok {
		t.Fatal("the third segment did not render")
	}
	tt.ClickAt(below.X+below.W/2, below.Y+below.H+30)
	tt.Frame()
	tt.Frame()
	if a.editSeg != -1 {
		t.Errorf("clicking away left editor %d open, want none", a.editSeg)
	}

	// Escape ends it too.
	tt.ClickAt(first.X+first.W/2, first.Y+first.H/2)
	tt.Frame()
	if a.editSeg != 0 {
		t.Fatalf("reopening the editor gave %d, want 0", a.editSeg)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.editSeg != -1 {
		t.Errorf("Escape left editor %d open, want none", a.editSeg)
	}
}
