package asr

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
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
}

// New returns a transcriber that submits audio through the cloudflared binary
// at cloudflared, or the default install location when that is empty.
func New(apiKey, cloudflared string) Transcriber {
	return Transcriber{apiKey: apiKey, cloudflared: cloudflared}
}

// Transcribe serves path over a tunnel, submits it, polls for the result and
// returns timed segments. duration backs a single fallback segment when the
// service returns no utterance timings.
func (t Transcriber) Transcribe(path string, duration float64) (*store.Transcript, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel() // also kills cloudflared

	port, route, shutdown, err := tunnel.StartFileServer(path)
	if err != nil {
		return nil, fmt.Errorf("启动本地文件服务失败：%w", err)
	}
	defer func() {
		sh, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = shutdown(sh)
	}()

	base, err := tunnel.StartTunnel(ctx, port, t.cloudflared)
	if err != nil {
		return nil, err
	}
	fileURL := base + route
	if err := tunnel.WaitTunnelReady(ctx, fileURL); err != nil {
		return nil, err
	}

	taskID, err := submitTask(t.apiKey, fileURL, detectFormat(path))
	if err != nil {
		return nil, err
	}
	body, err := queryResult(ctx, t.apiKey, taskID, 5*time.Second, 20*time.Minute)
	if err != nil {
		return nil, err
	}
	return parseTranscript(body, duration), nil
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
