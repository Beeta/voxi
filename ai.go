package main

import "strings"

// transcriptText flattens a transcript's segments into newline-joined text for
// the model. Returns "" when the recording has no transcript yet.
func transcriptText(rec Recording) string {
	if rec.Transcript == nil {
		return ""
	}
	var b strings.Builder
	for i, s := range rec.Transcript.Segments {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s.Text)
	}
	return b.String()
}

// llm builds a model client from the current settings; ok is false when the
// LLM is not fully configured yet.
func (a *app) llm() (llmClient, bool) {
	s := a.settings.snapshot()
	base := strings.TrimSpace(s.LLMBaseURL)
	key := strings.TrimSpace(s.LLMKey)
	model := strings.TrimSpace(s.LLMModel)
	if base == "" || key == "" || model == "" {
		return llmClient{}, false
	}
	return llmClient{baseURL: base, apiKey: key, model: model}, true
}

// setStatus records a one-off status message and refreshes the UI.
func (a *app) setStatus(msg string) {
	a.mu.Lock()
	a.status = msg
	a.mu.Unlock()
	a.ping()
}

// generateSummary runs the model in the background and stores the result.
func (a *app) generateSummary(rec Recording) {
	client, ok := a.llm()
	if !ok {
		a.setStatus("请先在设置里配置大模型")
		return
	}
	a.mu.Lock()
	if a.summarizing[rec.ID] {
		a.mu.Unlock()
		return
	}
	a.summarizing[rec.ID] = true
	a.mu.Unlock()
	a.ping()
	prompt := rec.SummaryPrompt
	if prompt == "" {
		prompt = defaultSummaryPrompt
	}
	text := transcriptText(rec)
	go func() {
		sm, err := client.Summarize(text, prompt)
		a.mu.Lock()
		delete(a.summarizing, rec.ID)
		if err != nil {
			a.status = err.Error()
		}
		a.mu.Unlock()
		if err == nil {
			a.store.setSummary(rec.ID, sm)
		}
		a.ping()
	}()
}

// generateMindmap runs the model in the background and stores the outline.
func (a *app) generateMindmap(rec Recording) {
	client, ok := a.llm()
	if !ok {
		a.setStatus("请先在设置里配置大模型")
		return
	}
	a.mu.Lock()
	if a.mindmapping[rec.ID] {
		a.mu.Unlock()
		return
	}
	a.mindmapping[rec.ID] = true
	a.mu.Unlock()
	a.ping()
	text := transcriptText(rec)
	go func() {
		root, err := client.Mindmap(text)
		a.mu.Lock()
		delete(a.mindmapping, rec.ID)
		if err != nil {
			a.status = err.Error()
		}
		a.mu.Unlock()
		if err == nil {
			a.store.setMindmap(rec.ID, &Mindmap{Root: root})
		}
		a.ping()
	}()
}

// ask appends the question, then streams the model's answer into the chat.
func (a *app) ask(rec Recording, question string) {
	question = strings.TrimSpace(question)
	if question == "" {
		return
	}
	client, ok := a.llm()
	if !ok {
		a.setStatus("请先在设置里配置大模型")
		return
	}
	a.mu.Lock()
	if a.asking[rec.ID] {
		a.mu.Unlock()
		return
	}
	a.asking[rec.ID] = true
	a.mu.Unlock()
	text := transcriptText(rec)
	history := rec.Chat
	a.store.appendChat(rec.ID, ChatMessage{Role: "user", Text: question})
	a.store.appendChat(rec.ID, ChatMessage{Role: "assistant", Text: ""})
	a.ping()
	go func() {
		var acc strings.Builder
		_, err := client.Ask(text, history, question, func(delta string) {
			acc.WriteString(delta)
			a.store.streamLastAssistant(rec.ID, acc.String())
			a.ping()
		})
		if err != nil {
			a.store.streamLastAssistant(rec.ID, "出错了："+err.Error())
		}
		a.store.commitChat(rec.ID)
		a.mu.Lock()
		delete(a.asking, rec.ID)
		a.mu.Unlock()
		a.ping()
	}()
}
