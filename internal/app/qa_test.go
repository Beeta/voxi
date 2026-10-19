package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/store"
)

// qaApp is an app whose model is srv, holding one transcribed recording on
// the Q&A tab.
func qaApp(t *testing.T, srv *httptest.Server) (*app, store.Recording) {
	t.Helper()
	a, set := newSettingsApp(t)
	set.update(func(s *Settings) {
		s.LLMProviders = []LLMProvider{{ID: "p", Name: "测试", BaseURL: srv.URL, Key: "k", Models: []string{"m"}}}
	})
	rec := a.lib.Add("问答测试", filepath.Join(t.TempDir(), "x.m4a"), 1)
	a.lib.SetTranscript(rec.ID, &store.Transcript{Segments: []store.Segment{{Text: "今天讨论排期"}}})
	rec, _ = a.lib.Get(rec.ID)
	a.tab = 3
	return a, rec
}

// sse writes chunks as an OpenAI streaming reply.
func sse(w http.ResponseWriter, chunks ...string) {
	for _, ch := range chunks {
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", ch)
		w.(http.Flusher).Flush()
	}
}

// waitFor frames tt until cond holds.
func waitFor(t *testing.T, tt *ui.Tester, cond func() bool) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	t.Fatalf("timed out; the screen shows %q", tt.Texts())
}

// TestQAAnswerIsMarkdownAndCopies: an answer is drawn as Markdown (no
// literal ** on screen), 复制 puts the source on the clipboard, and a 新话题
// keeps the earlier turns out of the next request.
func TestQAAnswerIsMarkdownAndCopies(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<16)
		n, _ := r.Body.Read(buf)
		bodies = append(bodies, string(buf[:n]))
		sse(w, "**重点**：", "排期")
	}))
	defer srv.Close()
	a, rec := qaApp(t, srv)
	view := func(c *ui.Context) {
		r, _ := a.lib.Get(rec.ID)
		a.qaView(c, r)
	}
	tt := ui.NewTester(view, 900, 700)
	a.ask(rec, "第一个问题")
	waitFor(t, tt, func() bool { return tt.HasText("复制") })
	if tt.HasText("**重点**：排期") {
		t.Error("the answer is shown as raw Markdown")
	}
	if err := tt.Click("复制"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != "**重点**：排期" {
		t.Errorf("复制 copied %q", tt.Clipboard())
	}

	if err := tt.Click("新话题"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r, _ := a.lib.Get(rec.ID)
	a.ask(r, "第二个问题")
	waitFor(t, tt, func() bool { r, _ := a.lib.Get(rec.ID); return len(r.Chat) == 5 && !a.isAsking(rec.ID) })
	if len(bodies) != 2 || strings.Contains(bodies[1], "第一个问题") {
		t.Errorf("after 新话题 the request still carries the first question: %s", bodies[len(bodies)-1])
	}
}

// TestQAStopKeepsWhatArrived stops an answer midway: the part streamed so
// far stays, marked as stopped, and the tab is free for the next question.
func TestQAStopKeepsWhatArrived(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, "已经说了一半")
		<-r.Context().Done()
	}))
	defer srv.Close()
	a, rec := qaApp(t, srv)
	tt := ui.NewTester(func(c *ui.Context) { r, _ := a.lib.Get(rec.ID); a.qaView(c, r) }, 900, 700)
	a.ask(rec, "问")
	waitFor(t, tt, func() bool { return tt.HasText("已经说了一半") })
	if err := tt.Click("停止生成"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, func() bool { return !a.isAsking(rec.ID) })
	r, _ := a.lib.Get(rec.ID)
	last := r.Chat[len(r.Chat)-1]
	if !strings.HasPrefix(last.Text, "已经说了一半") || !strings.Contains(last.Text, "已停止") || last.Error {
		t.Errorf("after stopping, the answer is %+v", last)
	}
}

// TestQAArchiveRestoreDelete drives the toolbar: 归档 empties the chat into
// 历史会话, which lists it; 恢复 brings it back; 删除 a turn removes both
// halves.
func TestQAArchiveRestoreDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sse(w, "答") }))
	defer srv.Close()
	a, rec := qaApp(t, srv)
	a.lib.AppendChat(rec.ID, store.ChatMessage{Role: "user", Text: "要归档的问题"})
	a.lib.AppendChat(rec.ID, store.ChatMessage{Role: "assistant", Text: "答"})
	tt := ui.NewTester(func(c *ui.Context) { r, _ := a.lib.Get(rec.ID); a.qaView(c, r) }, 900, 700)
	tt.Frame()
	if err := tt.Click("归档"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("要归档的问题") {
		t.Fatal("the archived chat is still on screen")
	}
	if err := tt.Click("历史会话（1）"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("要归档的问题") {
		t.Fatalf("历史会话 does not list it: %q", tt.Texts())
	}
	if err := tt.Click("恢复为当前对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if r, _ := a.lib.Get(rec.ID); len(r.Chat) != 0 || len(r.ChatArchive) != 0 {
		t.Errorf("after restore + delete: chat %+v, archive %d", r.Chat, len(r.ChatArchive))
	}
}
