//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAACEncoderSmoke encodes a second of synthetic stereo audio and checks
// that a non-trivial .m4a (an MP4 'ftyp' file) comes out, exercising the
// ExtAudioFile FFI without the GUI.
func TestAACEncoderSmoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "smoke.m4a")
	enc, err := newAACEncoder(path, aacBitRate)
	if err != nil {
		t.Fatalf("newAACEncoder: %v", err)
	}

	// One second of white noise (incompressible, so the encoder runs near
	// its target bit rate) delivered in 10 ms spans like capture does.
	const span = sampleRate / 100
	var seed uint32 = 1
	for i := 0; i < 100; i++ {
		buf := make([]float32, span*channels)
		for j := range buf {
			seed = seed*1664525 + 1013904223
			buf[j] = float32(seed)/float32(1<<31) - 1
		}
		if err := enc.write(buf); err != nil {
			t.Fatalf("write span %d: %v", i, err)
		}
	}
	if err := enc.close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() < 2<<10 {
		t.Fatalf("output too small: %d bytes", info.Size())
	}
	head, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(head) < 12 || string(head[4:8]) != "ftyp" {
		t.Fatalf("not an MP4/.m4a container: % x", head[:min(12, len(head))])
	}
	t.Logf("encoded %d bytes to %s", info.Size(), path)
}
