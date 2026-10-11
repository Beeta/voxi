//go:build !darwin

package main

import "errors"

// newAACEncoder is unavailable off macOS; audio capture is unsupported there
// too, so this exists only to keep the package building.
func newAACEncoder(path string, bitrate int) (encoder, error) {
	return nil, errors.New("AAC 编码仅支持 macOS")
}
