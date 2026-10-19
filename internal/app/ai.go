package app

import (
	"context"
	"errors"
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

// ask sends question about rec to the model and streams the answer into a
// new turn. The history sent is the current topic only (store.ChatContext).
func (a *app) ask(rec store.Recording, question string) {
	question = strings.TrimSpace(question)
	if question == "" {
		return
	}
	history := store.ChatContext(rec.Chat)
	if !a.startAnswer(rec.ID, func() bool {
		a.lib.AppendChat(rec.ID, store.ChatMessage{Role: "user", Text: question})
		a.lib.AppendChat(rec.ID, store.ChatMessage{Role: "assistant"})
		return true
	}) {
		return
	}
	a.streamAnswer(rec, history, question)
}

// regenerate answers the last question again, in place of its answer.
func (a *app) regenerate(rec store.Recording) {
	n := len(rec.Chat)
	if n < 2 || rec.Chat[n-1].Role != "assistant" || rec.Chat[n-2].Role != "user" {
		return
	}
	question := rec.Chat[n-2].Text
	history := store.ChatContext(rec.Chat[:n-2])
	if !a.startAnswer(rec.ID, func() bool { return a.lib.ResetLastAnswer(rec.ID) }) {
		return
	}
	a.streamAnswer(rec, history, question)
}

// startAnswer claims rec's Q&A for one answer, running prepare (which adds
// or resets the answer turn) under the claim; false when the model is not
// configured, an answer is already streaming, or prepare found nothing to
// answer, in which case the claim is let go again.
func (a *app) startAnswer(id string, prepare func() bool) bool {
	if _, ok := a.llm(); !ok {
		a.setStatus("请先在设置里配置大模型")
		return false
	}
	a.mu.Lock()
	if a.asking[id] {
		a.mu.Unlock()
		return false
	}
	a.asking[id] = true
	a.mu.Unlock()
	if !prepare() {
		a.mu.Lock()
		delete(a.asking, id)
		a.mu.Unlock()
		return false
	}
	a.ping()
	return true
}

// streamAnswer runs the request in the background, filling the last answer
// as it streams. stopAnswer cancels it, keeping what has arrived.
func (a *app) streamAnswer(rec store.Recording, history []store.ChatMessage, question string) {
	client, _ := a.llm()
	text := transcriptText(rec)
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.askCancel[rec.ID] = cancel
	a.mu.Unlock()
	go func() {
		defer cancel()
		var acc strings.Builder
		_, err := client.Ask(ctx, text, history, question, func(delta string) {
			acc.WriteString(delta)
			a.lib.StreamLastAssistant(rec.ID, acc.String())
			a.ping()
		})
		out, failed := acc.String(), false
		switch {
		case errors.Is(err, context.Canceled):
			if strings.TrimSpace(out) == "" {
				out, failed = "（已停止）", true
			} else {
				out += "\n\n*（已停止）*"
			}
		case err != nil:
			out, failed = "出错了："+err.Error(), true
		}
		a.lib.FinishLastAssistant(rec.ID, out, failed)
		a.mu.Lock()
		delete(a.asking, rec.ID)
		delete(a.askCancel, rec.ID)
		a.mu.Unlock()
		a.ping()
	}()
}

// stopAnswer cancels the answer streaming for id, if any.
func (a *app) stopAnswer(id string) {
	a.mu.Lock()
	cancel := a.askCancel[id]
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// isAsking reports whether an answer is streaming for id.
func (a *app) isAsking(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.asking[id]
}
