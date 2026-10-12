package llm

import (
	"strings"
	"testing"
)

// TestSummarizeInstructionReachesTheSystemMessage pins where the instruction
// goes. It used to be pasted in front of the user turn, with the transcript
// behind it — which can run to tens of thousands of words. An instruction
// there reads as context, and the model follows it far less reliably than the
// schema it was meant to override.
func TestSummarizeInstructionReachesTheSystemMessage(t *testing.T) {
	const instruction = "生成会议纪要，按议题分组，标出每个议题的结论"
	const transcript = "转写正文"

	msgs := summarizeMessages(transcript, instruction)
	if len(msgs) != 2 {
		t.Fatalf("summarizeMessages returned %d messages, want 2", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatalf("the first message has role %q, want the system one", msgs[0].Role)
	}
	if !strings.Contains(msgs[0].Content, instruction) {
		t.Errorf("the system message does not carry the instruction:\n%q", msgs[0].Content)
	}
	if msgs[1].Role != "user" {
		t.Errorf("the second message has role %q, want the user one", msgs[1].Role)
	}
	if !strings.Contains(msgs[1].Content, transcript) {
		t.Errorf("the user message does not carry the transcript:\n%q", msgs[1].Content)
	}
	if strings.Contains(msgs[1].Content, instruction) {
		t.Error("the instruction is still in the user turn, ahead of the transcript")
	}
	// The schema stays: the summary tab renders these three fields, so an
	// arbitrary shape would have nowhere to go.
	for _, field := range []string{"overview", "highlights", "actions"} {
		if !strings.Contains(msgs[0].Content, field) {
			t.Errorf("the system message dropped %q from the schema:\n%q", field, msgs[0].Content)
		}
	}
}

func TestSummarizeFallsBackToTheDefaultInstruction(t *testing.T) {
	msgs := summarizeMessages("转写正文", "")
	if !strings.Contains(msgs[0].Content, DefaultSummaryPrompt) {
		t.Errorf("an empty instruction did not fall back to the default:\n%q", msgs[0].Content)
	}
}
