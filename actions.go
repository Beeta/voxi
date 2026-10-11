package main

import "time"

func (a *app) ping() {
	a.mu.Lock()
	notify := a.notify
	a.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// tick refreshes the window while recording or playing so the timer and the
// playback position advance.
func (a *app) tick() {
	for range time.Tick(250 * time.Millisecond) {
		a.mu.Lock()
		p, recording, notify := a.player, a.recording, a.notify
		a.mu.Unlock()
		if notify != nil && (recording || (p != nil && p.isPlaying())) {
			notify()
		}
	}
}

func (a *app) selectRec(id string) {
	a.mu.Lock()
	a.selected = id
	a.mu.Unlock()
	a.editSeg = -1
	a.ping()
}

// ensurePlayer makes a.player point at rec, creating it on first use and
// replacing it when the selection changes.
func (a *app) ensurePlayer(rec Recording) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.playerID == rec.ID && a.player != nil {
		return
	}
	if a.player != nil {
		a.player.close()
		a.player = nil
	}
	p, err := newPlayer(rec.File)
	if err != nil {
		a.status, a.playerID = err.Error(), ""
		return
	}
	a.player, a.playerID, a.playPos = p, rec.ID, 0
}

func (a *app) togglePlay() {
	a.mu.Lock()
	p := a.player
	a.mu.Unlock()
	if p == nil {
		return
	}
	if p.isPlaying() {
		p.pause()
	} else {
		p.play()
	}
	a.ping()
}

func (a *app) seekTo(sec float64) {
	a.mu.Lock()
	p := a.player
	a.playPos = sec
	a.mu.Unlock()
	if p == nil {
		return
	}
	p.seek(sec)
	if !p.isPlaying() {
		p.play()
	}
	a.ping()
}
