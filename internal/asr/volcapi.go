package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Beeta/voxi/internal/tunnel"
)

const volcResourceID = "volc.seedasr.auc" // 豆包录音文件识别模型 2.0

// The service's endpoints; variables so tests can point them at a fake.
var (
	volcSubmitURL = "https://openspeech.bytedance.com/api/v3/auc/bigmodel/submit"
	volcQueryURL  = "https://openspeech.bytedance.com/api/v3/auc/bigmodel/query"
)

// Status codes the query answers with while the task is not done yet.
const (
	volcQueued     = "20000001"
	volcProcessing = "20000002"
)

type volcAudio struct {
	Format string `json:"format,omitempty"`
	URL    string `json:"url,omitempty"`
}

type volcRequest struct {
	ModelName         string `json:"model_name"`
	EnableITN         bool   `json:"enable_itn"`
	EnablePunc        bool   `json:"enable_punc"`
	EnableDDC         bool   `json:"enable_ddc"`
	ShowUtterances    bool   `json:"show_utterances"`
	EnableSpeakerInfo bool   `json:"enable_speaker_info"`
}

type volcSubmit struct {
	User    map[string]string `json:"user"`
	Audio   volcAudio         `json:"audio"`
	Request volcRequest       `json:"request"`
}

// submitTask submits a transcription task and returns its task id.
func submitTask(apiKey, fileURL, format string) (string, error) {
	reqID, err := tunnel.NewUUID()
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(volcSubmit{
		User:  map[string]string{"uid": "voxi"},
		Audio: volcAudio{Format: format, URL: fileURL},
		Request: volcRequest{
			ModelName: "bigmodel", EnableITN: true, EnablePunc: true,
			EnableDDC: true, ShowUtterances: true, EnableSpeakerInfo: true,
		},
	})
	req, err := http.NewRequest(http.MethodPost, volcSubmitURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("X-Api-Resource-Id", volcResourceID)
	req.Header.Set("X-Api-Request-Id", reqID)
	req.Header.Set("X-Api-Sequence", "-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	status := resp.Header.Get("X-Api-Status-Code")
	msg := resp.Header.Get("X-Api-Message")
	if status != "20000000" && !strings.EqualFold(msg, "OK") {
		return "", fmt.Errorf("提交失败：status=%s message=%s", status, msg)
	}
	var parsed struct {
		TaskID string `json:"task_id"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	if parsed.TaskID != "" {
		return parsed.TaskID, nil
	}
	return reqID, nil
}

// queryResult polls until the task finishes, returning the raw result JSON.
// onPoll, which may be nil, hears each answer that is not the result yet:
// whether the task is still queued (or already processing).
func queryResult(ctx context.Context, apiKey, taskID string, interval, timeout time.Duration, onPoll func(queued bool)) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("查询超时（%s），任务仍未完成", timeout)
		}
		req, err := http.NewRequest(http.MethodPost, volcQueryURL, bytes.NewReader([]byte(`{}`)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", apiKey)
		req.Header.Set("X-Api-Resource-Id", volcResourceID)
		req.Header.Set("X-Api-Request-Id", taskID)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		switch resp.Header.Get("X-Api-Status-Code") {
		case "20000000":
			return respBody, nil
		case volcQueued, volcProcessing:
			if onPoll != nil {
				onPoll(resp.Header.Get("X-Api-Status-Code") == volcQueued)
			}
			select {
			case <-time.After(interval):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		default:
			return nil, fmt.Errorf("查询失败：status=%s message=%s",
				resp.Header.Get("X-Api-Status-Code"), resp.Header.Get("X-Api-Message"))
		}
	}
}
