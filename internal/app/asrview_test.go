package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beeta/voxi/internal/asr"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// asrApp is an app over one recording with audio, whose ASR is fn.
func asrApp(t *testing.T, fn func(on func(asr.Progress)) (*store.Transcript, error)) (*app, store.Recording) {
	t.Helper()
	dir := t.TempDir()
	lib := openTestLib(t, dir)
	file := filepath.Join(dir, "m.m4a")
	os.WriteFile(file, []byte("x"), 0o644)
	rec := lib.Add("会议", file, 60)
	set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &Settings{ASRKey: "k"}}
	a := newApp(lib, set, audio.NewSimulatedRecorder())
	a.runASR = func(_, _, _ string, _ float64, on func(asr.Progress)) (*store.Transcript, error) { return fn(on) }
	return a, rec
}

// at is a Progress through step s: the ones before it done, s under way.
func at(s asr.Step) asr.Progress {
	t0 := time.Now().Add(-10 * time.Second)
	p := asr.Progress{Started: t0, Size: 2 << 20}
	for i := asr.Step(0); i < s; i++ {
		p.Steps[i] = asr.StepState{Started: t0, Ended: t0.Add(time.Second)}
	}
	p.Steps[s].Started = t0.Add(2 * time.Second)
	return p
}

// TestTranscriptTabShowsTheSteps: while transcribing, the tab lists the
// steps with the one under way and its detail, and the sidebar row names
// the step; when it is done the transcript takes their place.
func TestTranscriptTabShowsTheSteps(t *testing.T) {
	step, release := make(chan asr.Progress), make(chan struct{})
	a, rec := asrApp(t, func(on func(asr.Progress)) (*store.Transcript, error) {
		for p := range step {
			on(p)
		}
		<-release
		return &store.Transcript{Segments: []store.Segment{{Text: "转写好了"}}}, nil
	})
	a.selected = rec.ID
	tt := ui.NewTester(a.view, 968, 704)
	tt.Frame()
	a.transcribe(rec)

	p := at(asr.StepWait)
	p.Steps[asr.StepTunnel].Detail = "abc.trycloudflare.com"
	p.TaskID, p.Polls, p.Queued, p.Served = "task-xyz", 7, true, 1<<20
	step <- p
	close(step)
	// The report lands on the ASR goroutine; wait for it before drawing,
	// or the frame may be built half before it and half after.
	for i := 0; i < 200 && a.asrStatus(rec.ID) != "识别中 · 等待识别结果"; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	tt.Frame()
	for _, want := range []string{"识别中 · 已用时", "建立临时隧道", "abc.trycloudflare.com", "等待识别结果",
		"排队中 · 已查询 7 次 · 音频已被下载 1.0 MB/2.0 MB", "task-xyz", "识别中 · 等待识别结果"} {
		if !hasTextPart(tt, want) {
			t.Errorf("the page does not show %q; it shows %q", want, tt.Texts())
		}
	}
	if err := tt.Click("复制"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != "task-xyz" {
		t.Errorf("复制 copied %q", tt.Clipboard())
	}

	close(release)
	for i := 0; i < 200 && a.asrStatus(rec.ID) != ""; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	tt.Frame()
	if !tt.HasText("转写好了") || hasTextPart(tt, "等待识别结果") {
		t.Errorf("after finishing the page shows %q", tt.Texts())
	}
}

// TestFailedTranscriptionKeepsItsSteps: a failure leaves the steps on the
// page with the failed one and its reason, and asr.log says the same.
func TestFailedTranscriptionKeepsItsSteps(t *testing.T) {
	a, rec := asrApp(t, func(on func(asr.Progress)) (*store.Transcript, error) {
		p := at(asr.StepProbe)
		p.Steps[asr.StepProbe].Ended = time.Now()
		p.Steps[asr.StepProbe].Err = "隧道地址在限定时间内不可达（最后状态：HTTP 530）"
		on(p)
		return nil, errors.New(p.Steps[asr.StepProbe].Err)
	})
	a.selected = rec.ID
	tt := ui.NewTester(a.view, 968, 704)
	a.transcribe(rec)
	for i := 0; i < 200 && a.asrStatus(rec.ID) != ""; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	tt.Frame()
	for _, want := range []string{"识别失败", "隧道可达", "HTTP 530"} {
		if !hasTextPart(tt, want) {
			t.Errorf("the failed page does not show %q: %q", want, tt.Texts())
		}
	}
	dir, _ := mygo.App.Path(mygo.PathUserData)
	log, err := os.ReadFile(filepath.Join(dir, "asr.log"))
	if err != nil {
		t.Fatal(err)
	}
	if s := string(log); !strings.Contains(s, "✕ 隧道可达") || !strings.Contains(s, "==== 失败") {
		t.Errorf("asr.log holds:\n%s", s)
	}
}

func hasTextPart(tt *ui.Tester, s string) bool {
	for _, t := range tt.Texts() {
		if strings.Contains(t, s) {
			return true
		}
	}
	return false
}
