package app

import (
	"strings"

	"github.com/Beeta/voxi/internal/llm"
	"github.com/Beeta/voxi/internal/store"
)

// transcriptText flattens a transcript's segments into newline-joined text for
// the model, prefixing each line with its speaker ("说话人1：…") when the ASR
// labelled one, so the summary and Q&A can tell who said what and attribute
// action items. Segments without a speaker are written as the text alone.
// Returns "" when the recording has no transcript yet.
func transcriptText(rec store.Recording) string {
	if rec.Transcript == nil {
		return ""
	}
	var b strings.Builder
	for i, s := range rec.Transcript.Segments {
		if i > 0 {
			b.WriteByte('\n')
		}
		if sp := strings.TrimSpace(s.Speaker); sp != "" {
			b.WriteString(sp)
			b.WriteString("：")
		}
		b.WriteString(s.Text)
	}
	return b.String()
}

// llm builds a model client for the picked provider × model; ok is false when the
// LLM is not fully configured yet.
func (a *app) llm() (llm.Client, bool) {
	o, ok := activeLLM(a.settings.snapshot())
	if !ok {
		return llm.Client{}, false
	}
	base := strings.TrimSpace(o.Provider.BaseURL)
	key := strings.TrimSpace(o.Provider.Key)
	if base == "" || key == "" {
		return llm.Client{}, false
	}
	return llm.New(base, key, o.Model), true
}

// setStatus records a one-off status message and refreshes the UI.
func (a *app) setStatus(msg string) {
	a.mu.Lock()
	a.status = msg
	a.mu.Unlock()
	a.ping()
}

// generateSummary runs the model in the background and stores the result.
func (a *app) generateSummary(rec store.Recording) {
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
		prompt = llm.DefaultSummaryPrompt
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
			a.lib.SetSummary(rec.ID, sm)
		}
		a.ping()
	}()
}

// generateMindmap runs the model in the background and stores the outline.
func (a *app) generateMindmap(rec store.Recording) {
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
			a.lib.SetMindmap(rec.ID, &store.Mindmap{Root: root})
		}
		a.ping()
	}()
}

// ask appends the question, then streams the model's answer into the chat.
func (a *app) ask(rec store.Recording, question string) {
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
	a.lib.AppendChat(rec.ID, store.ChatMessage{Role: "user", Text: question})
	a.lib.AppendChat(rec.ID, store.ChatMessage{Role: "assistant", Text: ""})
	a.ping()
	go func() {
		var acc strings.Builder
		_, err := client.Ask(text, history, question, func(delta string) {
			acc.WriteString(delta)
			a.lib.StreamLastAssistant(rec.ID, acc.String())
			a.ping()
		})
		if err != nil {
			a.lib.StreamLastAssistant(rec.ID, "出错了："+err.Error())
		}
		a.lib.CommitChat(rec.ID)
		a.mu.Lock()
		delete(a.asking, rec.ID)
		a.mu.Unlock()
		a.ping()
	}()
}
