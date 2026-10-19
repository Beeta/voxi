package app

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
)

// TestPlayerBarActionFollowsTab is the contract of the moved buttons: the
// player bar's trailing action is the current tab's, changing as the tab
// does, and the tabs without an action (Q&A, notes) add none. The recording
// is transcribed, so the transcript tab offers 重新识别 rather than 开始识别.
func TestPlayerBarActionFollowsTab(t *testing.T) {
	dir := t.TempDir()
	lib := openTestLib(t, dir)
	rec := lib.Add("会议", filepath.Join(dir, "m.m4a"), 60)
	lib.SetTranscript(rec.ID, &store.Transcript{
		Segments: []store.Segment{{Start: 0, End: 3, Text: "你好"}},
	})
	rec, _ = lib.Get(rec.ID)
	set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &Settings{}}
	a := newApp(lib, set, audio.NewRecorder())

	actions := []string{"重新识别", "生成摘要", "生成脑图"}
	for tab, want := range actions {
		a.tab = tab
		tt := ui.NewTester(func(c *ui.Context) { a.tabAction(c, rec) }, 400, 80)
		tt.Frame()
		if !tt.HasText(want) {
			t.Errorf("tab %d: the player bar does not show %q", tab, want)
		}
		for _, other := range actions {
			if other != want && tt.HasText(other) {
				t.Errorf("tab %d shows %q, which belongs to another tab", tab, other)
			}
		}
	}

	// Q&A (tab 3) and notes (tab 4) have no player-bar action.
	for _, tab := range []int{3, 4} {
		a.tab = tab
		tt := ui.NewTester(func(c *ui.Context) { a.tabAction(c, rec) }, 400, 80)
		tt.Frame()
		for _, label := range []string{"开始识别", "重新识别", "生成摘要", "生成脑图"} {
			if tt.HasText(label) {
				t.Errorf("tab %d has no action, but shows %q", tab, label)
			}
		}
	}
}

// TestSpeakerTintIndexes covers how the transcript picks each speaker's
// color: one number per distinct speaker, handed out in the order they first
// speak, so two speakers in one recording never come out the same color —
// which is the whole point of tinting the chips.
func TestSpeakerTintIndexes(t *testing.T) {
	rec := store.Recording{Transcript: &store.Transcript{Segments: []store.Segment{
		{Speaker: "甲"},
		{Speaker: ""}, // no speaker: draws no chip
		{Speaker: "乙"},
		{Speaker: "甲"}, // seen already: keeps its number
		{Speaker: "丙"},
	}}}
	got := speakerTintIndexes(rec)
	want := map[string]int{"甲": 0, "乙": 1, "丙": 2}
	if len(got) != len(want) {
		t.Fatalf("numbered %v, want %v", got, want)
	}
	for name, n := range want {
		if got[name] != n {
			t.Errorf("%s got tint %d, want %d", name, got[name], n)
		}
	}
	// An unnamed segment must take no number: counting it would shift every
	// later speaker onto the next color.
	if _, ok := got[""]; ok {
		t.Error("the segment with no speaker was numbered")
	}
	if n := len(speakerTintIndexes(store.Recording{})); n != 0 {
		t.Errorf("a recording with no transcript numbered %d speakers", n)
	}
}
