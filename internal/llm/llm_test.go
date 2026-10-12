package llm

import (
	"strings"
	"testing"
)

// TestSummarizeSendsTheInstructionVerbatim pins the contract: the instruction
// is the whole system message, with nothing appended, and the transcript is
// the whole user message. What the model is asked for is exactly what the
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
	if msgs[1].Role != "user" {
		t.Errorf("the second message has role %q, want the user one", msgs[1].Role)
	}
	if msgs[1].Content != transcript {
		t.Errorf("the user message is %q, want the transcript alone", msgs[1].Content)
	}
}

// TestSummarizeFallsBackToTheDefaultInstruction covers an empty instruction.
// The default carries the output schema itself, so a user who never opens the
// editor still gets the tab's three sections rather than one block of text.
func TestSummarizeFallsBackToTheDefaultInstruction(t *testing.T) {
	msgs := summarizeMessages("转写正文", "")
	if msgs[0].Content != DefaultSummaryPrompt {
		t.Errorf("an empty instruction did not fall back to the default:\n%q", msgs[0].Content)
	}
	for _, field := range []string{"overview", "highlights", "actions"} {
		if !strings.Contains(DefaultSummaryPrompt, field) {
			t.Errorf("the default instruction does not mention %q, so the summary tab would show one block of text", field)
		}
	}
}
