package store

import (
	"reflect"
	"testing"
)

func q(t string) ChatMessage { return ChatMessage{Role: "user", Text: t} }
func ans(t string) ChatMessage { return ChatMessage{Role: "assistant", Text: t} }

var div = ChatMessage{Role: ChatDivider}

// chatOf is a store holding one recording with the given chat.
func chatOf(t *testing.T, chat ...ChatMessage) (*Store, string) {
	t.Helper()
	s, _ := newTestStore(t)
	rec := s.Add("会议", "", 1)
	s.recs[0].Chat = chat
	return s, rec.ID
}

func roles(chat []ChatMessage) []string {
	var out []string
	for _, m := range chat {
		out = append(out, m.Role+":"+m.Text)
	}
	return out
}

// TestChatContext: only the turns after the last divider go to the model,
// and failed or unanswered ones are left out, the old "出错了：" answers
// included.
func TestChatContext(t *testing.T) {
	chat := []ChatMessage{
		q("旧问题"), ans("旧回答"), div,
		q("好问题"), ans("好回答"),
		q("坏问题"), {Role: "assistant", Text: "超时", Error: true},
		q("老错误"), ans("出错了：HTTP 500"),
		q("没回答"),
	}
	got := roles(ChatContext(chat))
	want := []string{"user:好问题", "assistant:好回答"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("context %q, want %q", got, want)
	}
}

// TestDeleteChat removes a whole turn whichever half is clicked, and drops
// dividers left with nothing to separate.
func TestDeleteChat(t *testing.T) {
	s, id := chatOf(t, q("一"), ans("1"), div, q("二"), ans("2"))
	s.DeleteChat(id, 4) // the answer takes its question with it
	if got := roles(s.recs[0].Chat); !reflect.DeepEqual(got, []string{"user:一", "assistant:1"}) {
		t.Errorf("after deleting answer 2: %q", got)
	}
	s.DeleteChat(id, 0)
	if len(s.recs[0].Chat) != 0 {
		t.Errorf("after deleting question 1: %q", roles(s.recs[0].Chat))
	}
	s.DeleteChat(id, 5) // out of range is a no-op, not a panic
}

// TestArchiveAndRestore: archiving keeps the conversation as a titled
// session and leaves an empty one; restoring brings it back and archives
// what was current, so switching never loses a conversation.
func TestArchiveAndRestore(t *testing.T) {
	s, id := chatOf(t, q("第一个问题\n第二行"), ans("答"), div)
	s.ArchiveChat(id)
	r := s.recs[0]
	if len(r.Chat) != 0 || len(r.ChatArchive) != 1 {
		t.Fatalf("after archiving: chat %d, archive %d", len(r.Chat), len(r.ChatArchive))
	}
	sess := r.ChatArchive[0]
	if sess.Title != "第一个问题" || len(sess.Messages) != 2 {
		t.Errorf("archived %q with %d messages (the trailing divider should go)", sess.Title, len(sess.Messages))
	}
	s.ArchiveChat(id) // nothing to archive
	if len(s.recs[0].ChatArchive) != 1 {
		t.Error("archiving an empty chat made a session")
	}

	s.AppendChat(id, q("新的"))
	s.AppendChat(id, ans("新答"))
	s.RestoreChat(id, sess.ID)
	r = s.recs[0]
	if got := roles(r.Chat); !reflect.DeepEqual(got, []string{"user:第一个问题\n第二行", "assistant:答"}) {
		t.Errorf("restored %q", got)
	}
	if len(r.ChatArchive) != 1 || r.ChatArchive[0].Title != "新的" {
		t.Errorf("the replaced conversation was not archived: %+v", r.ChatArchive)
	}
	s.DeleteChatSession(id, r.ChatArchive[0].ID)
	if len(s.recs[0].ChatArchive) != 0 {
		t.Error("the session was not deleted")
	}
}

// TestNewChatTopicAndRegenerate: a divider only goes after something, once;
// ResetLastAnswer only applies to a final answered question.
func TestNewChatTopicAndRegenerate(t *testing.T) {
	s, id := chatOf(t)
	s.NewChatTopic(id)
	if len(s.recs[0].Chat) != 0 {
		t.Error("a divider was added to an empty chat")
	}
	s.AppendChat(id, q("问"))
	s.AppendChat(id, ans("答"))
	if !s.ResetLastAnswer(id) || s.recs[0].Chat[1].Text != "" {
		t.Error("the last answer was not reset")
	}
	s.NewChatTopic(id)
	s.NewChatTopic(id)
	if n := len(s.recs[0].Chat); n != 3 {
		t.Errorf("two 新话题 in a row gave %d messages, want 3", n)
	}
	if s.ResetLastAnswer(id) {
		t.Error("an answer was reset across a divider")
	}
}

// TestCutTitle keeps titles to one short line.
func TestCutTitle(t *testing.T) {
	if got := cutTitle("  一二三四五六  ", 4); got != "一二三四…" {
		t.Errorf("got %q", got)
	}
}
