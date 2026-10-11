//go:build !darwin

package audio

import "errors"

// player is unavailable off macOS, where there is no capture either.
type Player struct{}

func NewPlayer(path string) (*Player, error) { return nil, errors.New("音频播放仅支持 macOS") }

func (p *Player) Play()                {}
func (p *Player) Pause()               {}
func (p *Player) IsPlaying() bool      { return false }
func (p *Player) Duration() float64    { return 0 }
func (p *Player) CurrentTime() float64 { return 0 }
func (p *Player) Seek(sec float64)     {}
func (p *Player) Close()               {}
