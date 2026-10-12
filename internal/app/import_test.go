package app

import (
	"errors"
	"strings"
	"testing"
)

// TestImportable pins the format list to what both halves of the app accept.
// The player opens flac, aiff, opus and caf as well, and the ASR service takes
// spx, but neither of those overlaps is importable: the player would not play
// an spx back, and asr.detectFormat does not know the others and would submit
// them as m4a. Widening the list means teaching the other side first.
func TestImportable(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"meeting.m4a", true},
		{"meeting.MP3", true}, // the extension's case does not matter
		{"meeting.wav", true},
		{"meeting.aac", true},
		{"meeting.ogg", true},
		{"meeting.amr", true},
		{"/Users/x/录音 01.wav", true},

		{"meeting.flac", false}, // plays, but detectFormat would call it m4a
		{"meeting.aiff", false},
		{"meeting.opus", false},
		{"meeting.caf", false},
		{"meeting.spx", false}, // detectFormat knows it, the player does not
		{"meeting.mp4", false},
		{"meeting", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := importable(tc.path); got != tc.want {
			t.Errorf("importable(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestImportName(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/Users/x/会议记录.mp3", "会议记录"},
		{"standup.wav", "standup"},
		{"a.b.m4a", "a.b"}, // only the last extension goes
		{"/Users/x/录音 01.ogg", "录音 01"},
	}
	for _, tc := range cases {
		if got := importName(tc.path); got != tc.want {
			t.Errorf("importName(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
	// A file named just ".m4a" has nothing left to show, so the name falls
	// back to a timestamp rather than an empty row.
	if got := importName(".m4a"); got == "" {
		t.Error("importName(\".m4a\") is empty, want a fallback name")
	}
}

// TestGuardTurnsPanicIntoError covers the boundary that keeps a background
// task's panic from taking the window down with it.
func TestGuardTurnsPanicIntoError(t *testing.T) {
	err := guard(func() error { panic("boom") })
	if err == nil {
		t.Fatal("guard returned nil for a panicking function")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("guard returned %q, want it to carry the panic's text", err)
	}
	if err := guard(func() error { return nil }); err != nil {
		t.Errorf("guard returned %v for a function that did not panic", err)
	}
	sentinel := errors.New("asr down")
	if err := guard(func() error { return sentinel }); !errors.Is(err, sentinel) {
		t.Errorf("guard returned %v, want the function's own error through", err)
	}
}
