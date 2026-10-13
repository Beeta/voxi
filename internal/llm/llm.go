package llm

import (
	"encoding/json"
	"strings"

	"github.com/Beeta/voxi/internal/store"
)

// DefaultSummaryPrompt is the editable instruction shown in the summary tab.
// It asks for a cleaned-up, structured meeting minute in Markdown, which the
// tab draws as one Markdown document — headings, lists and the action-item
// table included (see summaryBody). Nothing is appended to the instruction on
// the way out, so the output shape is whatever it asks for: only a reply that
// happens to parse as the summary's JSON shape is drawn as three sections.
const DefaultSummaryPrompt = `# Role: 你是一位专业的高级项目经理和会议记录专家，具备出色的文本清洗和信息提炼能力。
# Task: 请对以下由语音识别生成的原始会议转写文本进行**脏数据清理**，并提炼成一份结构清晰、重点突出的正式会议纪要。

## 会议背景：
[简述会议主题，例如：Q3产品规划评审会]

## 一、 脏数据清洗规范（请在提炼前按此规范处理文本）：
1. **去除口语与语气词：** 过滤掉"嗯"、"啊"、"这个"、"那个"、"然后"、"就是说"、"对对对"等口语化废话和语气词。
2. **修正语音识别错误：** 根据上下文逻辑自动修正明显由 ASR（语音识别）产生的同音错别字或断句错误。
3. **合并重复表达：** 剔除发言者的口吃、重复发言、卡顿以及毫无实际意义的闲聊寒暄。
4. **优化表述：** 将分散、断续的口语化表达重构为通顺、严谨的书面语，同时严格保留原意，不捏造事实。

## 二、 输出格式要求：
1. **会议基本信息**（会议主题、核心目标）
2. **核心决议/结论**（简明扼要列出达成的共识或做出的决策，无需复述讨论过程）
3. **行动项（Action Items）**（请使用表格列出：` + "`任务内容 | 负责人 | 截止时间`" + `）
4. **主要讨论点与议题分歧**（分条列出主要议题、各方关键观点及未决事项）
5. **会议金句与核心洞察**（提炼会议中最具战略启发性、高总结度或代表性的发言/金句，并注明发言人；若无则标注"无"）

## 三、 约束条件：
- 语言风格需保持客观、专业、精炼。
- 如果文本中缺少某个要素（如具体负责人或截止时间），请直接标注为"未明确"。`

// Client talks to an OpenAI-compatible chat-completions endpoint configured
// in settings (Base URL / API Key / model).
type Client struct {
	baseURL string
	apiKey  string
	model   string
}

// New returns a client for the given OpenAI-compatible endpoint.
func New(baseURL, apiKey, model string) Client {
	return Client{baseURL: baseURL, apiKey: apiKey, model: model}
}

// Summarize condenses the transcript with the caller's instruction. That
// instruction is the whole system message — nothing is appended to it — so the
// output shape is whatever it asks for. A reply that parses as the summary's
// JSON shape fills its three sections; anything else is shown as one block of
// text.
func (c Client) Summarize(transcript, prompt string) (*store.Summary, error) {
	out, err := c.complete(summarizeMessages(transcript, prompt))
	if err != nil {
		return nil, err
	}
	var sm store.Summary
	if e := json.Unmarshal([]byte(extractJSON(out)), &sm); e != nil || sm.Overview == "" {
		return &store.Summary{Overview: strings.TrimSpace(out)}, nil
	}
	return &sm, nil
}

// summarizeMessages builds the chat messages for Summarize: the instruction
// verbatim as the system message, then the transcript, labelled, as the user
// one.
func summarizeMessages(transcript, prompt string) []chatMessage {
	if prompt == "" {
		prompt = DefaultSummaryPrompt
	}
	return []chatMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: "转写内容：\n" + transcript},
	}
}

// Mindmap turns the transcript into an outline tree via a JSON reply.
func (c Client) Mindmap(transcript string) (*store.MindNode, error) {
	sys := "你是会议记录助手。把内容整理成层级大纲，只输出 JSON，不要解释、不要代码块标记，模式为：" +
		`{"text": "根节点", "children": [{"text": "分支", "children": [{"text": "要点"}]}]}`
	out, err := c.complete([]chatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: "转写内容：\n" + transcript},
	})
	if err != nil {
		return nil, err
	}
	var root store.MindNode
	if e := json.Unmarshal([]byte(extractJSON(out)), &root); e != nil || root.Text == "" {
		return &store.MindNode{Text: "脑图", Children: []*store.MindNode{{Text: strings.TrimSpace(out)}}}, nil
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
