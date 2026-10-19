package asr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beeta/voxi/internal/store"
	"github.com/Beeta/voxi/internal/tunnel"
)

// Transcriber transcribes a recording with Volc Engine's auc (豆包语音)
// file-recognition model. The model fetches the audio from a public URL, so
// the file is briefly exposed through a cloudflared quick tunnel.
type Transcriber struct {
	apiKey      string
	cloudflared string
	tunnel      tunnelOps
	interval    time.Duration // between result queries; 0 is the default
}

// New returns a transcriber that submits audio through the cloudflared binary
// at cloudflared, or the default install location when that is empty.
func New(apiKey, cloudflared string) Transcriber {
	return Transcriber{apiKey: apiKey, cloudflared: cloudflared, tunnel: realTunnel}
}

// Transcribe serves path over a tunnel, submits it, polls for the result and
// returns timed segments. duration backs a single fallback segment when the
// service returns no utterance timings. onProgress, which may be nil, hears
// every change of step and detail as it happens, from the goroutines doing
// the work; the last one it hears is the final state, failed step included.
func (t Transcriber) Transcribe(path string, duration float64, onProgress func(Progress)) (*store.Transcript, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel() // also kills cloudflared
	tr := newTracker(onProgress)
	if fi, err := os.Stat(path); err == nil {
		tr.update(func(p *Progress) { p.Size = fi.Size() })
	}

	tr.begin(StepServe)
	port, route, shutdown, err := t.tunnel.serve(path, func(n int64) {
		tr.update(func(p *Progress) { p.Served += n })
	})
	if err != nil {
		return nil, tr.end(StepServe, fmt.Errorf("启动本地文件服务失败：%w", err))
	}
	tr.end(StepServe, nil)
	defer func() {
		sh, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = shutdown(sh)
	}()

	tr.begin(StepTunnel)
	base, err := t.tunnel.start(ctx, port, t.cloudflared)
	if err != nil {
		return nil, tr.end(StepTunnel, err)
	}
	if u, err := url.Parse(base); err == nil {
		tr.detail(StepTunnel, u.Host) // the host only: the path's token keeps the file private
	}
	tr.end(StepTunnel, nil)

	tr.begin(StepProbe)
	fileURL := base + route
	err = t.tunnel.wait(ctx, fileURL, func(n int, last string) {
		d := "第 " + strconv.Itoa(n) + " 次探测"
		if last != "" {
			d += "（上次：" + last + "）"
		}
		tr.detail(StepProbe, d)
	})
	if err != nil {
		return nil, tr.end(StepProbe, err)
	}
	tr.end(StepProbe, nil)

	tr.begin(StepSubmit)
	taskID, err := submitTask(t.apiKey, fileURL, detectFormat(path))
	if err != nil {
		return nil, tr.end(StepSubmit, err)
	}
	tr.update(func(p *Progress) { p.TaskID = taskID })
	tr.end(StepSubmit, nil)

	tr.begin(StepWait)
	body, err := queryResult(ctx, t.apiKey, taskID, t.pollInterval(), 20*time.Minute, func(queued bool) {
		tr.update(func(p *Progress) { p.Polls++; p.Queued = queued })
	})
	if err != nil {
		return nil, tr.end(StepWait, err)
	}
	tr.update(func(p *Progress) { p.Polls++; p.Queued = false })
	tr.end(StepWait, nil)

	tr.begin(StepParse)
	ts := parseTranscript(body, duration)
	tr.detail(StepParse, strconv.Itoa(len(ts.Segments))+" 段")
	tr.end(StepParse, nil)
	return ts, nil
}

// tunnelOps are the steps that expose the file, so tests can do without
// cloudflared and the public internet.
type tunnelOps struct {
	serve func(path string, onSent func(int64)) (int, string, func(context.Context) error, error)
	start func(ctx context.Context, port int, cloudflared string) (string, error)
	wait  func(ctx context.Context, fileURL string, onAttempt func(int, string)) error
}

// realTunnel exposes the file through a cloudflared quick tunnel.
var realTunnel = tunnelOps{
	serve: tunnel.StartWatchedFileServer,
	start: tunnel.StartTunnel,
	wait:  tunnel.WaitTunnelReadyWatched,
}

// pollInterval is how often the result is queried.
func (t Transcriber) pollInterval() time.Duration {
	if t.interval > 0 {
		return t.interval
	}
	return 5 * time.Second
}

// detectFormat maps a file extension to Volc's format field.
func detectFormat(path string) string {
	switch ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")); ext {
	case "wav", "mp3", "ogg", "spx", "amr", "aac", "m4a":
		return ext
	default:
		return "m4a"
	}
}

// parseTranscript extracts timed segments from a Volc result: result.utterances
// (text + start_time/end_time in ms); failing that, one segment holding
// result.text across the whole duration.
func parseTranscript(body []byte, duration float64) *store.Transcript {
	var r struct {
		Result struct {
			Text       string `json:"text"`
			Utterances []struct {
				Text      string  `json:"text"`
				Start     float64 `json:"start_time"`
				End       float64 `json:"end_time"`
				Additions struct {
					Speaker string `json:"speaker"`
				} `json:"additions"`
			} `json:"utterances"`
		} `json:"result"`
	}
	_ = json.Unmarshal(body, &r)
	var segs []store.Segment
	for _, u := range r.Result.Utterances {
		text := strings.TrimSpace(u.Text)
		if text == "" {
			continue
		}
		speaker := strings.TrimSpace(u.Additions.Speaker)
		if speaker != "" {
			speaker = "说话人" + speaker
		}
		segs = append(segs, store.Segment{Start: u.Start / 1000, End: u.End / 1000, Text: text, Speaker: speaker})
	}
	if len(segs) == 0 {
		text := strings.TrimSpace(r.Result.Text)
		if text == "" {
			text = "（未识别到语音内容）"
		}
		segs = []store.Segment{{Start: 0, End: duration, Text: text}}
	}
	return &store.Transcript{Segments: segs, Language: "zh-CN"}
}
