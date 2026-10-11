//go:build darwin

package main

// Audio playback on macOS through AVAudioPlayer, driven by purego like the
// rest of this example. It reuses the Objective-C helpers from
// capture_darwin.go (same package) and adds float-typed objc_msgSend wrappers:
// currentTime/duration/setCurrentTime: move doubles through floating-point
// registers, which the integer-only send/sendBool helpers cannot carry.

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	loadPlayerOnce sync.Once
	msgFloat       func(obj id, sel objc.SEL) float64
	msgSetFloat    func(obj id, sel objc.SEL, v float64)
)

// loadPlayer registers the float message wrappers against the shared
// objc_msgSend address, once load() has resolved it.
func loadPlayer() error {
	if err := load(); err != nil {
		return err
	}
	loadPlayerOnce.Do(func() {
		purego.RegisterFunc(&msgFloat, msgSendAddr)
		purego.RegisterFunc(&msgSetFloat, msgSendAddr)
	})
	return nil
}

// player plays one audio file via a retained AVAudioPlayer. Its methods are
// safe to call from any goroutine; the UI polls currentTime/isPlaying.
type player struct {
	obj id
}

// newPlayer opens path for playback.
func newPlayer(path string) (*player, error) {
	if err := loadPlayer(); err != nil {
		return nil, err
	}
	url := send(class("NSURL"), "fileURLWithPath:", uintptr(nsString(path)))
	if url == 0 {
		return nil, errors.New("无法创建音频 URL")
	}
	var nsErr id
	obj := send(send(class("AVAudioPlayer"), "alloc"),
		"initWithContentsOfURL:error:", uintptr(url), uintptr(unsafe.Pointer(&nsErr)))
	if obj == 0 {
		return nil, errors.New("无法打开音频：" + nsErrorString(nsErr))
	}
	sendBool(obj, "prepareToPlay")
	return &player{obj: obj}, nil
}

func (p *player) play()           { sendBool(p.obj, "play") }
func (p *player) pause()          { send(p.obj, "pause") }
func (p *player) isPlaying() bool { return sendBool(p.obj, "isPlaying") }

func (p *player) duration() float64    { return msgFloat(p.obj, objc.RegisterName("duration")) }
func (p *player) currentTime() float64 { return msgFloat(p.obj, objc.RegisterName("currentTime")) }

func (p *player) seek(sec float64) {
	if sec < 0 {
		sec = 0
	}
	msgSetFloat(p.obj, objc.RegisterName("setCurrentTime:"), sec)
}

// close stops playback and releases the player.
func (p *player) close() {
	if p.obj != 0 {
		send(p.obj, "stop")
		send(p.obj, "release")
		p.obj = 0
	}
}
