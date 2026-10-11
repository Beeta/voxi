//go:build darwin

package audio

import (
	"path/filepath"
	"testing"
)

// TestPlayerSmoke encodes a short clip, opens it with the AVAudioPlayer
// wrapper and exercises the float-typed msgSend path (duration/seek/
// currentTime) without producing sound.
func TestPlayerSmoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.m4a")
	enc, err := newAACEncoder(path, aacBitRate)
	if err != nil {
		t.Fatalf("newAACEncoder: %v", err)
	}
	const span = sampleRate / 100
	for i := 0; i < 100; i++ { // ~1 second of silence
		if err := enc.write(make([]float32, span*channels)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := enc.close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	p, err := NewPlayer(path)
	if err != nil {
		t.Fatalf("NewPlayer: %v", err)
	}
	defer p.Close()

	// duration reads a double back — the clearest check that the float
	// msgSend wrapper is wired correctly.
	if d := p.Duration(); d < 0.5 || d > 2.0 {
		t.Fatalf("duration = %v, want ~1s (float msgSend likely wrong)", d)
	}
	// seek passes a double in; currentTime reads one back. Values before
	// play() are not guaranteed, so just exercise the calls.
	p.Seek(0.5)
	_ = p.CurrentTime()
}
