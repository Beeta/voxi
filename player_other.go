//go:build !darwin

package main

import "errors"

// player is unavailable off macOS, where there is no capture either.
type player struct{}

func newPlayer(path string) (*player, error) { return nil, errors.New("音频播放仅支持 macOS") }

func (p *player) play()                {}
func (p *player) pause()               {}
func (p *player) isPlaying() bool      { return false }
func (p *player) duration() float64    { return 0 }
func (p *player) currentTime() float64 { return 0 }
func (p *player) seek(sec float64)     {}
func (p *player) close()               {}
