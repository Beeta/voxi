package main

import (
	"encoding/json"
	"strings"
)

// defaultSummaryPrompt is the editable instruction shown in the summary tab.
const defaultSummaryPrompt = "请用简洁的中文总结这段录音：先给一句话概览，再列出关键要点，最后列出需要跟进的待办事项。"

// llmClient talks to an OpenAI-compatible chat-completions endpoint configured
// in settings (Base URL / API Key / model).
type llmClient struct {
	baseURL string
	apiKey  string
	model   string
}

// Summarize condenses the transcript into overview/highlights/actions. It asks
// the model for JSON and parses it, falling back to the raw reply as overview.
func (c llmClient) Summarize(transcript, prompt string) (*Summary, error) {
	if prompt == "" {
		prompt = defaultSummaryPrompt
	}
	sys := "你是会议记录助手。只输出 JSON，不要解释、不要代码块标记，模式为：" +
		`{"overview": "一句话概览", "highlights": ["关键要点"], "actions": ["待办事项"]}`
	out, err := c.complete([]chatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: prompt + "\n\n转写内容：\n" + transcript},
	})
	if err != nil {
		return nil, err
	}
	var sm Summary
	if e := json.Unmarshal([]byte(extractJSON(out)), &sm); e != nil || sm.Overview == "" {
		return &Summary{Overview: strings.TrimSpace(out)}, nil
	}
	return &sm, nil
}

// Mindmap turns the transcript into an outline tree via a JSON reply.
func (c llmClient) Mindmap(transcript string) (*MindNode, error) {
	sys := "你是会议记录助手。把内容整理成层级大纲，只输出 JSON，不要解释、不要代码块标记，模式为：" +
		`{"text": "根节点", "children": [{"text": "分支", "children": [{"text": "要点"}]}]}`
	out, err := c.complete([]chatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: "转写内容：\n" + transcript},
	})
	if err != nil {
		return nil, err
	}
	var root MindNode
	if e := json.Unmarshal([]byte(extractJSON(out)), &root); e != nil || root.Text == "" {
		return &MindNode{Text: "脑图", Children: []*MindNode{{Text: strings.TrimSpace(out)}}}, nil
	}
	return &root, nil
}

// extractJSON pulls the first {...} block out of a reply, dropping any prose or
// ```json fences the model may wrap it in.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '{'); i >= 0 {
		if j := strings.LastIndexByte(s, '}'); j > i {
			return s[i : j+1]
		}
	}
	return s
}
