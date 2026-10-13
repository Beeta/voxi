package app

import (
	"testing"

	"github.com/Beeta/voxi/internal/store"
)

// TestTranscriptText pins the text handed to the model: each segment on its own
// line, prefixed with the speaker when the ASR labelled one so the summary and
// Q&A can attribute who said what, and the bare text when it did not.
func TestTranscriptText(t *testing.T) {
	withTranscript := func(segs ...store.Segment) store.Recording {
		return store.Recording{Transcript: &store.Transcript{Segments: segs}}
	}
	cases := []struct {
		name string
		rec  store.Recording
		want string
	}{
		{
			name: "no transcript yet",
			rec:  store.Recording{},
			want: "",
		},
		{
			name: "speakers are prefixed",
			rec: withTranscript(
				store.Segment{Speaker: "说话人1", Text: "今天同步下季度排期"},
				store.Segment{Speaker: "说话人2", Text: "我倾向先做移动端"},
			),
			want: "说话人1：今天同步下季度排期\n说话人2：我倾向先做移动端",
		},
		{
			name: "no speaker is bare text",
			rec: withTranscript(
				store.Segment{Text: "一段没有说话人的文字"},
				store.Segment{Text: "第二段"},
			),
			want: "一段没有说话人的文字\n第二段",
		},
		{
			name: "mixed: prefix only where there is a speaker",
			rec: withTranscript(
				store.Segment{Speaker: "张三", Text: "开场"},
				store.Segment{Text: "没有标注的一段"},
				store.Segment{Speaker: "  ", Text: "空白说话人按无处理"},
			),
			want: "张三：开场\n没有标注的一段\n空白说话人按无处理",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := transcriptText(tc.rec); got != tc.want {
				t.Errorf("transcriptText() = %q, want %q", got, tc.want)
			}
		})
	}
}
