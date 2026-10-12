package app

import (
	"testing"

	"github.com/Beeta/voxi/internal/store"
)

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
