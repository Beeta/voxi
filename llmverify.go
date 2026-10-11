package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// verifyLLM checks the configured endpoint in the background by sending one
// minimal chat request, then shows the result in the settings dialog.
func (a *app) verifyLLM() {
	s := a.settings.snapshot()
	base := strings.TrimSpace(s.LLMBaseURL)
	key := strings.TrimSpace(s.LLMKey)
	model := strings.TrimSpace(s.LLMModel)
	a.mu.Lock()
	if a.verifyingLLM {
		a.mu.Unlock()
		return
	}
	if base == "" || key == "" || model == "" {
		a.llmVerifyOK = false
		a.llmVerifyMsg = "请先填写 Base URL、API Key 和模型。"
		a.mu.Unlock()
		a.ping()
		return
	}
	a.verifyingLLM = true
	a.llmVerifyMsg = ""
	a.mu.Unlock()
	a.ping()
	go func() {
		err := verifyConnection(base, key, model)
		a.mu.Lock()
		a.verifyingLLM = false
		if err != nil {
			a.llmVerifyOK, a.llmVerifyMsg = false, "连接失败："+err.Error()
		} else {
			a.llmVerifyOK, a.llmVerifyMsg = true, "连接正常"
		}
		a.mu.Unlock()
		a.ping()
	}()
}

// verifyConnection POSTs a minimal OpenAI-compatible chat request and reports
// whether the endpoint accepted it.
func verifyConnection(baseURL, key, model string) error {
	url := chatCompletionsURL(baseURL)
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
