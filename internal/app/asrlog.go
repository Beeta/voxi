package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Beeta/voxi/internal/asr"
	"github.com/Beeta/voxi/internal/store"
	"github.com/egoist/mygo"
)

// asrLog appends one transcription's steps to asr.log in the app's data
// directory, so a failure can be looked at afterwards — which step, how
// long each took, what the service said — without the user retelling it.
// Only step changes are written, not every byte the service fetches, and
// nothing that would let someone fetch the audio (the tunnel's path).
type asrLog struct {
	mu    sync.Mutex
	f     *os.File
	start time.Time
	seen  [len(asr.StepNames)]struct{ started, ended bool }
}

// asrLogMax is how large asr.log may grow before it starts over.
const asrLogMax = 1 << 20

// newASRLog opens the log for one transcription of rec. A log that cannot
// be opened is no reason to stop transcribing: the methods then do nothing.
func newASRLog(rec store.Recording) *asrLog {
	l := &asrLog{start: time.Now()}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return l
	}
	path := filepath.Join(dir, "asr.log")
	if fi, err := os.Stat(path); err == nil && fi.Size() > asrLogMax {
		os.Remove(path)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return l
	}
	l.f = f
	l.printf("==== 开始识别「%s」 %s（%s）", rec.Name, filepath.Base(rec.File), formatSecs(rec.Duration))
	return l
}

func (l *asrLog) printf(format string, args ...any) {
	if l.f == nil {
		return
	}
	fmt.Fprintf(l.f, "%s +%5.1fs  %s\n", time.Now().Format("2006-01-02 15:04:05"),
		time.Since(l.start).Seconds(), fmt.Sprintf(format, args...))
}

// progress writes the steps that started or ended since the last call.
func (l *asrLog) progress(p asr.Progress) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, s := range p.Steps {
		seen := &l.seen[i]
		if !s.Started.IsZero() && !seen.started {
			seen.started = true
			l.printf("→ %s", asr.StepNames[i])
		}
		if !s.Ended.IsZero() && !seen.ended {
			seen.ended = true
			line := fmt.Sprintf("%s %s %.1fs", stepMark(s), asr.StepNames[i], s.Ended.Sub(s.Started).Seconds())
			if d := stepDetail(asr.Step(i), s, p); d != "" {
				line += "  " + d
			}
			if s.Err != "" {
				line += "  错误：" + s.Err
			}
			l.printf("%s", line)
		}
	}
}

// finish writes the outcome and closes the log.
func (l *asrLog) finish(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err != nil {
		l.printf("==== 失败：%s", strings.TrimSpace(err.Error()))
	} else {
		l.printf("==== 完成")
	}
	if l.f != nil {
		l.f.Close()
		l.f = nil
	}
}

func stepMark(s asr.StepState) string {
	if s.Err != "" {
		return "✕"
	}
	return "✓"
}
