package app

import (
	"strings"

	"github.com/Beeta/voxi/internal/llm"
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
		err := llm.Verify(base, key, model)
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
