package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Beeta/voxi/internal/store"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Endpoint normalizes a configured base URL to the chat-completions URL the
// client actually requests: trailing slashes are dropped, and /chat/completions
// is appended unless the base already ends with it. The settings dialog shows
// the result so a missing (or extra) /v1 is visible before it 404s.
func Endpoint(base string) string {
	u := strings.TrimRight(strings.TrimSpace(base), "/")
	if !strings.HasSuffix(u, "/chat/completions") {
		u += "/chat/completions"
	}
	return u
}

// complete runs a non-streaming chat completion and returns the reply text.
func (c Client) complete(msgs []chatMessage) (string, error) {
	body, _ := json.Marshal(map[string]any{"model": c.model, "messages": msgs, "stream": false})
	raw, err := c.post(body)
	if err != nil {
		return "", err
	}
	var r struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	if len(r.Choices) == 0 {
		return "", fmt.Errorf("模型未返回内容")
	}
	return r.Choices[0].Message.Content, nil
}

// post sends a request body to the endpoint and returns the response bytes,
// turning a non-200 into an error with a short body snippet.
func (c Client) post(body []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, Endpoint(c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := (&http.Client{Timeout: 180 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

// Ask streams an answer about the transcript, calling emit for each chunk.
func (c Client) Ask(transcript string, history []store.ChatMessage, question string, emit func(string)) (string, error) {
	msgs := []chatMessage{{Role: "system", Content: "你是会议记录助手，依据给定的转写内容回答问题；无法从内容得知时如实说明。\n\n转写内容：\n" + transcript}}
	for _, m := range history {
		msgs = append(msgs, chatMessage{Role: m.Role, Content: m.Text})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: question})
	body, _ := json.Marshal(map[string]any{"model": c.model, "messages": msgs, "stream": true})
	req, err := http.NewRequest(http.MethodPost, Endpoint(c.baseURL), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := (&http.Client{Timeout: 180 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var full strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		if d := chunk.Choices[0].Delta.Content; d != "" {
			full.WriteString(d)
			emit(d)
		}
	}
	return full.String(), sc.Err()
}

// Verify POSTs one minimal chat request to the OpenAI-compatible endpoint at
// baseURL and reports whether it accepted it. The settings dialog uses it to
// check a connection before saving.
func Verify(baseURL, key, model string) error {
	url := Endpoint(baseURL)
	payload, _ := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens": 1,
	})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	msg := strings.TrimSpace(string(snippet))
	if msg == "" {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return fmt.Errorf("HTTP %d：%s", resp.StatusCode, msg)
}

// ModelsEndpoint is the OpenAI-compatible model-list URL beside the
// chat-completions one Endpoint derives from base: the same root with
// /models in place of /chat/completions.
func ModelsEndpoint(base string) string {
	return strings.TrimSuffix(Endpoint(base), "/chat/completions") + "/models"
}

// ListModels asks the endpoint at baseURL which models it serves (GET
// /models), so the settings dialog can offer them instead of having the
// model ids typed from memory. The ids come back sorted.
func ListModels(baseURL, key string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, ModelsEndpoint(baseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		if msg == "" {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d：%s", resp.StatusCode, msg)
	}
	var r struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("返回的不是模型列表：%w", err)
	}
	var ids []string
	for _, m := range r.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("接口没有返回任何模型")
	}
	sort.Strings(ids)
	return ids, nil
}
