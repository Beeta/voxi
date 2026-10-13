package llm

import (
	"strings"
	"testing"
)

// TestSummarizeSendsTheInstructionVerbatim pins the contract: the instruction
// is the whole system message, with nothing appended, and the transcript
// follows in the user turn. What the model is asked for is exactly what the
// caller wrote.
func TestSummarizeSendsTheInstructionVerbatim(t *testing.T) {
	const instruction = "生成会议纪要，按议题分组，标出每个议题的结论"
	const transcript = "转写正文"

	msgs := summarizeMessages(transcript, instruction)
	if len(msgs) != 2 {
		t.Fatalf("summarizeMessages returned %d messages, want 2", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatalf("the first message has role %q, want the system one", msgs[0].Role)
	}
	if msgs[0].Content != instruction {
		t.Errorf("the system message is\n%q\nwant the instruction untouched:\n%q", msgs[0].Content, instruction)
	}
	if strings.Contains(msgs[0].Content, transcript) {
		t.Error("the transcript leaked into the system message")
	}
	if msgs[1].Role != "user" {
		t.Errorf("the second message has role %q, want the user one", msgs[1].Role)
	}
	if !strings.Contains(msgs[1].Content, transcript) {
		t.Errorf("the user message does not carry the transcript: %q", msgs[1].Content)
	}
}

// TestSummarizeFallsBackToTheDefaultInstruction covers an empty instruction:
// the default instruction is used verbatim.
func TestSummarizeFallsBackToTheDefaultInstruction(t *testing.T) {
	msgs := summarizeMessages("转写正文", "")
	if msgs[0].Content != DefaultSummaryPrompt {
		t.Errorf("an empty instruction did not fall back to the default:\n%q", msgs[0].Content)
	}
	if !strings.Contains(DefaultSummaryPrompt, "会议纪要") {
		t.Errorf("the default instruction no longer asks for a meeting minute:\n%q", DefaultSummaryPrompt)
	}
}
