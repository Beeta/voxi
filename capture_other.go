//go:build !darwin

package main

// On platforms other than macOS, no audio capture is implemented yet.
type unsupportedCapture struct{}

func newCapture() capture { return unsupportedCapture{} }

func (unsupportedCapture) available() (bool, string) {
	return false, "音频采集目前仅支持 macOS 15 及以上版本。"
}

func (unsupportedCapture) start([]Source, func(Source, chunk)) error { return nil }

func (unsupportedCapture) stop() {}
