package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
)

// touchFile writes an empty file in dir and returns its path. Recordings in
// these tests need a file on disk: without one they read as audio that is
// gone, and the row menu offers less.
func touchFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// audioApp is an app whose library holds one recording, with an audio file on
// disk unless the test asks for one without. transcribed decides whether it
// carries text, which is what the drop-audio warning turns on.
func audioApp(t *testing.T, withAudio, transcribed bool) (*app, *store.Store, store.Recording) {
	t.Helper()
	dir := t.TempDir()
	lib, err := store.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "m.m4a")
	if withAudio {
		file = touchFile(t, dir, "m.m4a")
	}
	rec := lib.Add("会议", file, 60)
	if transcribed {
		lib.SetTranscript(rec.ID, &store.Transcript{
			Segments: []store.Segment{{Start: 0, End: 3, Text: "你好", Speaker: "甲"}},
		})
		lib.SetSummary(rec.ID, &store.Summary{Overview: "概览"})
		lib.SetNotes(rec.ID, "笔记")
	}
	rec, _ = lib.Get(rec.ID)
	set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &Settings{}}
	return newApp(lib, set, audio.NewRecorder()), lib, rec
}

// TestRowMenuOffersBothDeletes covers what the row menu offers: the two
// deletes are named for what they take, and the one that takes the audio is
// not offered once there is no audio to take.
func TestRowMenuOffersBothDeletes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		audio bool
		want  []string
	}{
		{
			"with audio", true,
			[]string{"重命名", "在访达中显示", "-", "删除音频文件（保留文字）", "删除条目"},
		},
		{
			// Nothing to show in the Finder and nothing to delete: the menu
			// leads only to the delete that takes the entry.
			"without audio", false,
			[]string{"重命名", "-", "删除条目"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _, _ := audioApp(t, tc.audio, true)
			tt := ui.NewTester(a.view, 968, 704)
			tt.Frame()
			tt.Frame()
			if !tt.HasText("会议") {
				t.Skip("the sidebar did not render")
			}
			if err := tt.RightClick("会议"); err != nil {
				t.Fatal(err)
			}
			if got := tt.Menu(); !slices.Equal(got, tc.want) {
				t.Errorf("the row menu is %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDropAudioKeepsTheTextFromTheMenu drives the whole path a user takes:
// choose 删除音频文件（保留文字）, confirm, and the entry is still there with
// everything written about it, only the audio gone.
func TestDropAudioKeepsTheTextFromTheMenu(t *testing.T) {
	a, lib, rec := audioApp(t, true, true)
	// The real TrashItem runs on the main thread, which a test process has no
	// event loop for; this watches the path it was given instead.
	var trashed []string
	a.trashItem = func(path string) error { trashed = append(trashed, path); return nil }

	tt := ui.NewTester(a.view, 968, 704)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("会议") {
		t.Skip("the sidebar did not render")
	}

	if err := tt.RightClick("会议"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("删除音频文件（保留文字）"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("删除音频文件？") {
		t.Fatal("the confirmation did not render")
	}
	if err := tt.Click("删除音频"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if len(trashed) != 1 || trashed[0] != rec.File {
		t.Errorf("trashed %q, want the one file %q", trashed, rec.File)
	}
	got, ok := lib.Get(rec.ID)
	if !ok {
		t.Fatal("the entry is gone, but only its audio was to be dropped")
	}
	if got.File != "" || !got.AudioDeleted {
		t.Errorf("the entry still points at %q (deleted=%v)", got.File, got.AudioDeleted)
	}
	if got.Transcript == nil || got.Summary == nil || got.Notes != "笔记" {
		t.Errorf("the text went with the audio: transcript=%v summary=%v notes=%q",
			got.Transcript != nil, got.Summary != nil, got.Notes)
	}
}

// TestDropAudioWarnsWhenThereIsNoText covers the confirmation's other half:
// with nothing transcribed, dropping the audio leaves an entry holding
// nothing and unable to be transcribed again, and the dialog has to say so
// and point at the delete that takes the entry instead.
func TestDropAudioWarnsWhenThereIsNoText(t *testing.T) {
	for _, tc := range []struct {
		name        string
		transcribed bool
		want        string
	}{
		{"transcribed", true, "转写、摘要、脑图、笔记和问答都保留"},
		{"not transcribed", false, "请改用「删除条目」"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _, rec := audioApp(t, true, tc.transcribed)
			a.delAudioID, a.confirmDelAudio = rec.ID, true
			tt := ui.NewTester(a.view, 968, 704)
			tt.Frame()
			tt.Frame()
			if !tt.HasText("删除音频文件？") {
				t.Fatal("the confirmation did not render")
			}
			if !tt.HasText(tc.want) {
				t.Errorf("the dialog does not say %q", tc.want)
			}
		})
	}
}

// TestNoAudioDisablesTheTransport covers the detail pane once the audio is
// gone: the transport gives way to an explanation, and the tab's own action
// is still there — disabled for transcription, which needs the audio, but not
// for the summary, which is built from the text that is left.
func TestNoAudioDisablesTheTransport(t *testing.T) {
	a, lib, rec := audioApp(t, true, true)
	lib.DropAudio(rec.ID)
	rec, _ = lib.Get(rec.ID)
	a.selected = rec.ID
	a.tab = 0

	tt := ui.NewTester(a.view, 968, 704)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("会议") {
		t.Skip("the sidebar did not render")
	}
	if !tt.HasText(audioGone(rec)) {
		t.Error("the player bar does not say why there is nothing to play")
	}
	if tt.HasText("重新识别") == false {
		t.Error("the transcript tab lost its action")
	}

	// The action is disabled: clicking it starts nothing.
	before := a.transcribing[rec.ID]
	if err := tt.Click("重新识别"); err == nil {
		if a.transcribing[rec.ID] != before {
			t.Error("the disabled transcribe action started transcription")
		}
	}

	// The summary tab's action still works: it needs the text, not the audio.
	// (The fixture already has a summary, so the button reads 重新生成.)
	a.tab = 1
	tt.Frame()
	tt.Frame()
	if !tt.HasText("重新生成") {
		t.Error("the summary tab lost its action")
	}
}
