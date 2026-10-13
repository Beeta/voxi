package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDropAudioKeepsTheText covers dropping a recording's audio: the entry
// stays, everything written about it stays, and it stops pointing at a file
// that is no longer there.
func TestDropAudioKeepsTheText(t *testing.T) {
	s, root := newTestStore(t)
	file := touch(t, filepath.Join(root, "a.m4a"))
	rec := s.Add("会议", file, 60)
	s.SetTranscript(rec.ID, &Transcript{Segments: []Segment{{Start: 0, End: 3, Text: "你好"}}})
	s.SetSummary(rec.ID, &Summary{Overview: "概览"})
	s.SetNotes(rec.ID, "笔记")

	s.DropAudio(rec.ID)

	got, ok := s.Get(rec.ID)
	if !ok {
		t.Fatal("the entry is gone, but only its audio was dropped")
	}
	if got.File != "" {
		t.Errorf("the entry still points at %q", got.File)
	}
	if !got.AudioDeleted {
		t.Error("the entry does not record that its audio was dropped on purpose")
	}
	if got.Transcript == nil || got.Summary == nil || got.Notes != "笔记" {
		t.Errorf("the text went with the audio: transcript=%v summary=%v notes=%q",
			got.Transcript != nil, got.Summary != nil, got.Notes)
	}
	if len(s.Snapshot()) != 1 {
		t.Error("the entry left the library")
	}
}

// TestDropAudioSurvivesAReload keeps the dropped state across a restart:
// without the flag on disk, a relaunch would read the empty file reference as
// a file that went missing.
func TestDropAudioSurvivesAReload(t *testing.T) {
	s, root := newTestStore(t)
	file := touch(t, filepath.Join(root, "a.m4a"))
	rec := s.Add("会议", file, 60)
	s.DropAudio(rec.ID)

	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	var recs []Recording
	if err := json.Unmarshal(data, &recs); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || !recs[0].AudioDeleted || recs[0].File != "" {
		t.Errorf("the index reads back as %+v", recs)
	}
}

// TestMigrateToSkipsFilesThatAreGone covers the entry whose audio was dropped
// or removed in the Finder: the migration has nothing to move for it and must
// carry on, rather than failing and rolling back every file it already moved.
func TestMigrateToSkipsFilesThatAreGone(t *testing.T) {
	s, root := newTestStore(t)
	here := touch(t, filepath.Join(root, "here.m4a"))
	kept := s.Add("留下的", here, 1)

	gone := s.Add("没文件的", filepath.Join(root, "gone.m4a"), 1)
	dropped := s.Add("删过音频的", touch(t, filepath.Join(root, "dropped.m4a")), 1)
	s.DropAudio(dropped.ID)

	dest := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	n, err := s.MigrateTo(dest)
	if err != nil {
		t.Fatalf("migrating with a missing file failed: %v", err)
	}
	if n != 1 {
		t.Errorf("moved %d files, want the 1 that is still there", n)
	}
	if _, err := os.Stat(filepath.Join(dest, "here.m4a")); err != nil {
		t.Errorf("the file that exists did not move: %v", err)
	}
	for _, id := range []string{kept.ID, gone.ID, dropped.ID} {
		if _, ok := s.Get(id); !ok {
			t.Error("an entry went missing from the index")
		}
	}
}
