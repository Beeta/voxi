package asr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeVolc answers submit with a task id and the queries with queued,
// processing, then the result.
func fakeVolc(t *testing.T, answers ...string) {
	t.Helper()
	var mu sync.Mutex
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/submit") {
			w.Header().Set("X-Api-Status-Code", "20000000")
			w.Write([]byte(`{"task_id":"task-1"}`))
			return
		}
		mu.Lock()
		code := answers[min(n, len(answers)-1)]
		n++
		mu.Unlock()
		w.Header().Set("X-Api-Status-Code", code)
		if code == "20000000" {
			w.Write([]byte(`{"result":{"utterances":[{"text":"你好","start_time":0,"end_time":1000}]}}`))
		}
	}))
	t.Cleanup(srv.Close)
	oldS, oldQ := volcSubmitURL, volcQueryURL
	volcSubmitURL, volcQueryURL = srv.URL+"/submit", srv.URL+"/query"
	t.Cleanup(func() { volcSubmitURL, volcQueryURL = oldS, oldQ })
}

// fakeTunnel serves nothing over the internet: the "tunnel" is a made-up
// host, ready at the second probe, and the service is pretended to fetch
// the whole file as soon as the task is submitted.
func fakeTunnel(fail Step) tunnelOps {
	var sent func(int64)
	return tunnelOps{
		serve: func(path string, onSent func(int64)) (int, string, func(context.Context) error, error) {
			if fail == StepServe {
				return 0, "", nil, errors.New("端口被占用")
			}
			sent = onSent
			return 1, "/tok/audio.m4a", func(context.Context) error { return nil }, nil
		},
		start: func(context.Context, int, string) (string, error) {
			if fail == StepTunnel {
				return "", errors.New("找不到 cloudflared")
			}
			return "https://abc-def.trycloudflare.com", nil
		},
		wait: func(_ context.Context, _ string, onAttempt func(int, string)) error {
			onAttempt(1, "")
			onAttempt(2, "HTTP 530")
			if fail == StepProbe {
				return errors.New("隧道不可达")
			}
			sent(4)
			return nil
		},
	}
}

func audioFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.m4a")
	os.WriteFile(p, []byte("abcd"), 0o644)
	return p
}

// TestTranscribeReportsEveryStep runs a whole transcription against fakes
// and checks what the last report says about each step.
func TestTranscribeReportsEveryStep(t *testing.T) {
	fakeVolc(t, volcQueued, volcProcessing, "20000000")
	tr := Transcriber{apiKey: "k", tunnel: fakeTunnel(-1), interval: time.Millisecond}
	var mu sync.Mutex
	var reports []Progress
	ts, err := tr.Transcribe(audioFile(t), 1, func(p Progress) {
		mu.Lock()
		reports = append(reports, p)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ts.Segments) != 1 {
		t.Fatalf("segments %+v", ts.Segments)
	}
	last := reports[len(reports)-1]
	if last.Current() != stepCount {
		t.Errorf("the last report is at step %d, want all done", last.Current())
	}
	for i, s := range last.Steps {
		if s.Started.IsZero() || s.Ended.IsZero() || s.Err != "" {
			t.Errorf("step %s: %+v", StepNames[i], s)
		}
	}
	if last.Steps[StepTunnel].Detail != "abc-def.trycloudflare.com" {
		t.Errorf("the tunnel detail is %q, want the host alone", last.Steps[StepTunnel].Detail)
	}
	if !strings.Contains(last.Steps[StepProbe].Detail, "第 2 次") {
		t.Errorf("the probe detail is %q", last.Steps[StepProbe].Detail)
	}
	if last.TaskID != "task-1" || last.Polls != 3 || last.Served != 4 || last.Size != 4 {
		t.Errorf("task %q polls %d served %d/%d", last.TaskID, last.Polls, last.Served, last.Size)
	}
	sawQueued := false
	for _, p := range reports {
		if p.Queued {
			sawQueued = true
		}
	}
	if !sawQueued {
		t.Error("no report said the task was queued")
	}
}

// TestTranscribeFailureStopsAtItsStep: a failing step is marked with the
// reason, and the steps after it never start.
func TestTranscribeFailureStopsAtItsStep(t *testing.T) {
	for _, fail := range []Step{StepServe, StepTunnel, StepProbe} {
		var last Progress
		tr := Transcriber{apiKey: "k", tunnel: fakeTunnel(fail), interval: time.Millisecond}
		_, err := tr.Transcribe(audioFile(t), 1, func(p Progress) { last = p })
		if err == nil {
			t.Fatalf("%s: no error", StepNames[fail])
		}
		if last.Current() != fail || last.Steps[fail].Err == "" {
			t.Errorf("%s: the report stops at step %d with %+v", StepNames[fail], last.Current(), last.Steps[fail])
		}
		for s := fail + 1; s < stepCount; s++ {
			if !last.Steps[s].Started.IsZero() {
				t.Errorf("%s: step %s started after the failure", StepNames[fail], StepNames[s])
			}
		}
	}
}

// TestTranscribeQueryFailure: an error from the service while waiting is
// the waiting step's.
func TestTranscribeQueryFailure(t *testing.T) {
	fakeVolc(t, volcProcessing, "45000001")
	var last Progress
	tr := Transcriber{apiKey: "k", tunnel: fakeTunnel(-1), interval: time.Millisecond}
	if _, err := tr.Transcribe(audioFile(t), 1, func(p Progress) { last = p }); err == nil {
		t.Fatal("no error")
	}
	if last.Current() != StepWait || !strings.Contains(last.Steps[StepWait].Err, "45000001") {
		t.Errorf("stopped at %d: %+v", last.Current(), last.Steps[StepWait])
	}
}
