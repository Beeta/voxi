package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	a.recWarn = lowDiskWarning(filepath.Dir(filepath.Join(recordingsDir(set), "x")))
	a.mu.Unlock()
	a.rec.Start(srcs, filepath.Join(recordingsDir(set), defaultFilename()))
	a.ping()
}

// stopRecording ends capture, paused or not, and, once the file is written,
// adds it to the library and selects it. Its length is the time recorded,
// read once the recorder is idle: pauses are not in the file, so they must
// not be in the length either.
func (a *app) stopRecording() {
	_, _, path, _ := a.rec.Snapshot()
	a.rec.Stop()
	go func() {
		var elapsed time.Duration
		for {
			st, e, _, _ := a.rec.Snapshot()
			if st == audio.StateIdle {
				elapsed = e
				break
			}
			time.Sleep(80 * time.Millisecond)
		}
		a.mu.Lock()
		a.recording = false
		a.recWarn = ""
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
// the transcript back when it finishes. The audio is uploaded as-is. Every
// step is kept in a.asrProgress for the transcript tab to show, and written
// to the ASR log.
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
	a.asrProgress[rec.ID] = asr.Progress{Started: time.Now()}
	a.mu.Unlock()
	a.ping()
	go func() {
		log := newASRLog(rec)
		err := guard(func() error {
			t, err := a.runASR(key, set.CloudflaredPath, rec.File, rec.Duration, func(p asr.Progress) {
				log.progress(p)
				a.mu.Lock()
				a.asrProgress[rec.ID] = p
				a.mu.Unlock()
				a.ping()
			})
			if err != nil {
				return err
			}
			a.lib.SetTranscript(rec.ID, t)
			return nil
		})
		log.finish(err)
		a.mu.Lock()
		delete(a.transcribing, rec.ID)
		if err != nil {
			a.asrErr[rec.ID] = err.Error()
		} else {
			// Done: the transcript replaces the steps. A failure keeps them,
			// so the page still shows which step it was.
			delete(a.asrProgress, rec.ID)
		}
		a.mu.Unlock()
		a.ping()
	}()
}

// guard runs fn, turning a panic into an error. A background task that panics
// takes the whole window down with it and leaves the user nothing to report —
// no window, no message, no crash log. A recording whose file name held a
// space did exactly that to transcription, through the ServeMux pattern the
// name was pasted into. The panic's text is kept, so the bug still shows.
func guard(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("内部错误：%v", r)
		}
	}()
	return fn()
}

// doDelete trashes the audio file and drops the recording from the library.
func (a *app) doDelete(rec store.Recording) {
	a.closePlayerFor(rec.ID)
	a.trashItem(rec.File)
	a.lib.Remove(rec.ID)
	a.mu.Lock()
	// A row can be deleted from its context menu without being the
	// selection, so only step the selection forward when the recording
	// that just went away was the selected one.
	if a.selected == rec.ID {
		a.selected = ""
		if recs := a.lib.Snapshot(); len(recs) > 0 {
			a.selected = recs[0].ID
		}
	}
	a.mu.Unlock()
	a.ping()
}

// dropAudio trashes a recording's audio and keeps its text: the transcript,
// summary, mind map, notes and Q&A all stay, and the entry stays in the
// library.
func (a *app) dropAudio(rec store.Recording) {
	a.closePlayerFor(rec.ID)
	a.trashItem(rec.File)
	a.lib.DropAudio(rec.ID)
	a.ping()
}

// closePlayerFor lets go of the player if it is playing id.
func (a *app) closePlayerFor(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.playerID == id && a.player != nil {
		a.player.Close()
		a.player, a.playerID = nil, ""
	}
}

// hasAudio reports whether a recording still has an audio file to play. It
// answers the same for audio dropped on purpose and for a file that went
// missing on its own, which is what the views need; audioGone tells the two
// apart when saying so.
func hasAudio(rec store.Recording) bool {
	if rec.File == "" {
		return false
	}
	_, err := os.Stat(rec.File)
	return err == nil
}

// audioGone is why a recording has nothing to play, in the words of whoever
// lost it.
func audioGone(rec store.Recording) string {
	if rec.AudioDeleted {
		return "音频已删除，在废纸篓里；文字内容都保留着"
	}
	return "音频文件找不到了；文字内容都保留着"
}

// defaultFilename is a timestamped name for a new recording.
func defaultFilename() string {
	return fmt.Sprintf("recording-%s.m4a", time.Now().Format("20060102-150405"))
}

// diskFloor is how much free space recording wants: below it macOS starts
// purging caches under disk pressure, which stops every capture on the
// machine — this app's recording among them. A variable so a test can put
// the threshold on either side of the disk it runs on.
var diskFloor int64 = 15 << 30

// lowDiskWarning is what to tell the user before recording into dir, or ""
// when there is room. It is a warning, not a refusal: the recording still
// starts, with the stall watch behind it.
func lowDiskWarning(dir string) string {
	free, err := freeDisk(dir)
	if err != nil || free >= diskFloor {
		return ""
	}
	return fmt.Sprintf("磁盘仅剩 %.1f GB：系统清理缓存时可能中断录音（已自动看护，仍建议先腾空间）。",
		float64(free)/(1<<30))
}
