package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/Beeta/voxi/internal/asr"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
)

// startRecording begins capturing the chosen sources to a new file.
func (a *app) startRecording() {
	set := a.settings.snapshot()
	var srcs []audio.Source
	if set.Mic {
		srcs = append(srcs, audio.SourceMic)
	}
	if set.System {
		srcs = append(srcs, audio.SourceSystem)
	}
	if len(srcs) == 0 {
		return
	}
	a.mu.Lock()
	a.recording = true
	a.mu.Unlock()
	a.rec.Start(srcs, filepath.Join(recordingsDir(set), defaultFilename()))
	a.ping()
}

// stopRecording ends capture and, once the file is written, adds it to the
// library and selects it.
func (a *app) stopRecording() {
	_, elapsed, path, _ := a.rec.Snapshot()
	a.rec.Stop()
	go func() {
		for {
			st, _, _, _ := a.rec.Snapshot()
			if st == audio.StateIdle {
				break
			}
			time.Sleep(80 * time.Millisecond)
		}
		a.mu.Lock()
		a.recording = false
		a.mu.Unlock()
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				rec := a.lib.Add("录音 "+time.Now().Format("01-02 15:04"), path, elapsed.Seconds())
				a.mu.Lock()
				a.selected = rec.ID
				a.mu.Unlock()
			}
		}
		a.ping()
	}()
}

// transcribe runs ASR on the recording in the background with 豆包语音, writing
// the transcript back when it finishes. The audio is uploaded as-is.
func (a *app) transcribe(rec store.Recording) {
	set := a.settings.snapshot()
	key := strings.TrimSpace(set.ASRKey)
	a.mu.Lock()
	if a.transcribing[rec.ID] {
		a.mu.Unlock()
		return
	}
	if key == "" {
		a.asrErr[rec.ID] = "请先在设置里填写豆包语音 API Key"
		a.mu.Unlock()
		a.ping()
		return
	}
	delete(a.asrErr, rec.ID)
	a.transcribing[rec.ID] = true
	a.mu.Unlock()
	a.ping()
	go func() {
		t, err := asr.New(key, set.CloudflaredPath).Transcribe(rec.File, rec.Duration)
		a.mu.Lock()
		delete(a.transcribing, rec.ID)
		if err != nil {
			a.asrErr[rec.ID] = err.Error()
		}
		a.mu.Unlock()
		if err == nil {
			a.lib.SetTranscript(rec.ID, t)
		}
		a.ping()
	}()
}

// doDelete trashes the audio file and drops the recording from the library.
func (a *app) doDelete(rec store.Recording) {
	a.mu.Lock()
	if a.playerID == rec.ID && a.player != nil {
		a.player.Close()
		a.player, a.playerID = nil, ""
	}
	a.mu.Unlock()
	mygo.Shell.TrashItem(rec.File)
	a.lib.Remove(rec.ID)
	a.mu.Lock()
	a.selected = ""
	if recs := a.lib.Snapshot(); len(recs) > 0 {
		a.selected = recs[0].ID
	}
	a.mu.Unlock()
	a.ping()
}

// defaultFilename is a timestamped name for a new recording.
func defaultFilename() string {
	return fmt.Sprintf("recording-%s.m4a", time.Now().Format("20060102-150405"))
}
