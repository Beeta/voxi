package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatCompletionsURL normalizes a base URL to the chat-completions endpoint.
func chatCompletionsURL(base string) string {
	u := strings.TrimRight(strings.TrimSpace(base), "/")
	if !strings.HasSuffix(u, "/chat/completions") {
		u += "/chat/completions"
	}
	return u
}

// complete runs a non-streaming chat completion and returns the reply text.
func (c llmClient) complete(msgs []chatMessage) (string, error) {
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
func (c llmClient) post(body []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, chatCompletionsURL(c.baseURL), bytes.NewReader(body))
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
func (c llmClient) Ask(transcript string, history []ChatMessage, question string, emit func(string)) (string, error) {
	msgs := []chatMessage{{Role: "system", Content: "你是会议记录助手，依据给定的转写内容回答问题；无法从内容得知时如实说明。\n\n转写内容：\n" + transcript}}
	for _, m := range history {
		msgs = append(msgs, chatMessage{Role: m.Role, Content: m.Text})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: question})
	body, _ := json.Marshal(map[string]any{"model": c.model, "messages": msgs, "stream": true})
	req, err := http.NewRequest(http.MethodPost, chatCompletionsURL(c.baseURL), bytes.NewReader(body))
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
