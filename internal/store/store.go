package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// Segment is one timed span of transcript text.
type Segment struct {
	Start   float64 `json:"start"` // seconds from the start of the recording
	End     float64 `json:"end"`
	Text    string  `json:"text"`
	Speaker string  `json:"speaker,omitempty"` // speaker label when the ASR splits speakers
}

// Transcript is the ASR result for a recording: ordered timed segments. A nil
// Transcript means the recording has not been transcribed yet.
type Transcript struct {
	Segments []Segment `json:"segments"`
	Language string    `json:"language,omitempty"`
}

// Summary is the AI summary built from a transcript: a short overview plus key
// points and action items. Stale marks it out of date after a re-transcribe.
type Summary struct {
	Overview   string   `json:"overview"`
	Highlights []string `json:"highlights,omitempty"`
	Actions    []string `json:"actions,omitempty"`
	Stale      bool     `json:"stale,omitempty"`
}

// MindNode is one node of the mind-map outline tree.
type MindNode struct {
	Text     string      `json:"text"`
	Children []*MindNode `json:"children,omitempty"`
}

// Mindmap is the AI outline built from a transcript.
type Mindmap struct {
	Root  *MindNode `json:"root"`
	Stale bool      `json:"stale,omitempty"`
}

// ChatMessage is one turn of the transcript Q&A. Role is "user" or
// "assistant", or ChatDivider for a 新话题 line: the turns before the last
// divider stay on screen but are no longer sent to the model.
type ChatMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
	// Error marks an answer that is an error message rather than the
	// model's reply; it and its question are kept out of the context.
	Error bool `json:"error,omitempty"`
}

// ChatDivider is the Role of a 新话题 line.
const ChatDivider = "divider"

// ChatContext is what of chat the model is sent as history: the turns after
// the last divider, without unanswered questions and failed answers (an
// error message is not something the model said).
func ChatContext(chat []ChatMessage) []ChatMessage {
	start := 0
	for i, m := range chat {
		if m.Role == ChatDivider {
			start = i + 1
		}
	}
	var out []ChatMessage
	for i := start; i < len(chat); i++ {
		m := chat[i]
		if m.Role != "user" {
			continue
		}
		if i+1 < len(chat) && chat[i+1].Role == "assistant" && !failedAnswer(chat[i+1]) {
			out = append(out, m, chat[i+1])
		}
	}
	return out
}

// failedAnswer is an answer not worth sending back: empty, or an error.
// Answers from before ChatMessage.Error existed are recognised by the
// prefix the app wrote them with.
func failedAnswer(m ChatMessage) bool {
	t := strings.TrimSpace(m.Text)
	return m.Error || t == "" || strings.HasPrefix(t, "出错了：")
}

// ChatSession is an archived Q&A conversation.
type ChatSession struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"` // the first question, cut short
	Archived time.Time     `json:"archived"`
	Messages []ChatMessage `json:"messages"`
}

// Recording is one captured file and what the library knows about it.
type Recording struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	File    string    `json:"file"` // relative to the recordings directory, or absolute when outside it
	Created time.Time `json:"created"`
	// AudioDeleted marks a recording whose audio the user threw away on
	// purpose, keeping the text. File is empty with it, so the two together
	// say the entry never had audio (neither), lost it on purpose
	// (AudioDeleted), or lost it without anyone asking (File set, file
	// gone) — three cases the view says differently.
	AudioDeleted  bool          `json:"audioDeleted,omitempty"`
	Duration      float64       `json:"duration"` // seconds
	Transcript    *Transcript   `json:"transcript,omitempty"`
	Summary       *Summary      `json:"summary,omitempty"`
	SummaryPrompt string        `json:"summaryPrompt,omitempty"` // editable instruction; empty = default
	Mindmap       *Mindmap      `json:"mindmap,omitempty"`
	Chat          []ChatMessage `json:"chat,omitempty"`
	ChatArchive   []ChatSession `json:"chatArchive,omitempty"` // newest first
	Notes         string        `json:"notes,omitempty"`
}

// Store is the recording library and its JSON index on disk. It is safe for
// concurrent use: the UI reads snapshots while capture and ASR goroutines
// write.
//
// Recording.File is kept relative to root, the directory the audio lives in,
// so that moving that directory does not break the library. Files outside
// root — including ones recorded before the directory was configurable —
// keep their absolute path.
type Store struct {
	mu   sync.Mutex
	path string // recordings.json, in the app data directory
	root string // directory the recordings live in
	recs []*Recording
}

// OpenStore loads the library index from the app's data directory, returning
// an empty library when the file does not exist yet. root is the directory
// the recordings live in; it is not persisted here, so that settings.json can
// own it without the index and the settings referring to each other.
func OpenStore(root string) (*Store, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "recordings.json"), root: root}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &s.recs); err != nil {
		return nil, err
	}
	return s, nil
}

// Root returns the directory the recordings live in.
func (s *Store) Root() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.root
}

// SetRoot makes dir the recordings directory without moving anything. Only
// meaningful for callers that moved the files themselves; MigrateTo does both.
func (s *Store) SetRoot(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root = dir
}

// resolve turns a stored reference into an absolute path: a relative one is
// joined to the recordings directory, an absolute one is used as it is.
func (s *Store) resolve(file string) string {
	if file == "" || filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(s.root, file)
}

// relative stores file relative to the recordings directory when it lives
// there, so the library survives that directory moving. Files elsewhere keep
// their absolute path.
func (s *Store) relative(file string) string {
	if file == "" {
		return ""
	}
	rel, err := filepath.Rel(s.root, file)
	if err != nil || !isInside(rel) {
		return file
	}
	return rel
}

// external returns a copy of r with its file resolved to an absolute path.
func (s *Store) external(r Recording) Recording {
	r.File = s.resolve(r.File)
	return r
}

// MigrateTo moves every recording that lives under the current recordings
// directory into dir, then makes dir the new root. Recordings stored outside
// it are left where they are. If a move fails, the ones already moved are put
// back, so a failed migration leaves the library as it was. It returns how
// many files it moved.
func (s *Store) MigrateTo(dir string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if SameDir(dir, s.root) {
		return 0, nil
	}
	type moved struct {
		rec    *Recording
		from   string
		to     string
		oldRef string
	}
	var done []moved
	for _, r := range s.recs {
		abs := s.resolve(r.File)
		if abs == "" {
			continue // no audio to move
		}
		if _, err := os.Stat(abs); err != nil {
			// The file is gone — deleted on purpose, or removed in the
			// Finder. Skipping it keeps one missing file from failing the
			// whole migration and rolling back everything already moved.
			continue
		}
		if rel, err := filepath.Rel(s.root, abs); err != nil || !isInside(rel) {
			continue // not ours to move
		}
		dest := filepath.Join(dir, filepath.Base(abs))
		if err := moveFile(abs, dest); err != nil {
			for i := len(done) - 1; i >= 0; i-- { // put back what moved
				if moveFile(done[i].to, done[i].from) == nil {
					done[i].rec.File = done[i].oldRef
				}
			}
			return 0, fmt.Errorf("%s: %w", filepath.Base(abs), err)
		}
		oldRef := r.File
		r.File = filepath.Base(abs) // now relative to dir
		done = append(done, moved{r, abs, dest, oldRef})
	}
	s.root = dir
	s.saveLocked()
	return len(done), nil
}

// isInside reports whether a filepath.Rel result stays within the directory.
func isInside(rel string) bool {
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// SameDir reports whether two paths name the same directory, following
// symlinks and ignoring a trailing separator.
func SameDir(a, b string) bool {
	return canonicalDir(a) == canonicalDir(b)
}

// canonicalDir resolves dir to an absolute, symlink-free path, falling back
// to the cleaned input when it does not exist.
func canonicalDir(dir string) string {
	if p, err := filepath.EvalSymlinks(dir); err == nil {
		return p
	}
	if p, err := filepath.Abs(dir); err == nil {
		return p
	}
	return filepath.Clean(dir)
}

// moveFile moves src to dst. os.Rename only works within one volume, so a
// cross-volume move falls back to copying and removing the source.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyFile copies src to dst, replacing dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// ImportFile copies src into the recordings directory and returns the path to
// register, which Add turns into a library-relative reference. A file that
// already lives in that directory is returned as it is: importing what the
// library already holds should not put a second copy on disk. A name already
// taken gets " (n)" appended, since copying over an existing recording would
// leave two entries playing the same audio.
func (s *Store) ImportFile(src string) (string, error) {
	abs, err := filepath.Abs(src)
	if err != nil {
		abs = src
	}
	s.mu.Lock()
	root := s.root
	s.mu.Unlock()
	if rel, err := filepath.Rel(root, abs); err == nil && isInside(rel) {
		return abs, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	dst := uniquePath(filepath.Join(root, filepath.Base(abs)))
	if err := copyFile(abs, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// uniquePath returns path, or path with " (n)" before its extension when that
// is already taken.
func uniquePath(path string) string {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	candidate := path
	for i := 1; i <= 999; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
	}
	return candidate
}

// snapshot returns a copy of the recordings, newest first, so the UI can
// render without holding the lock. File is resolved to an absolute path.
func (s *Store) Snapshot() []Recording {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Recording, len(s.recs))
	for i, r := range s.recs {
		out[i] = s.external(*r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

// get returns a copy of one recording, with File resolved to an absolute path.
func (s *Store) Get(id string) (Recording, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		return s.external(*r), true
	}
	return Recording{}, false
}

// add registers a newly captured file and returns its record. file is an
// absolute path; it is stored relative to the recordings directory.
func (s *Store) Add(name, file string, duration float64) Recording {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := &Recording{
		ID:       fmt.Sprintf("rec-%d", time.Now().UnixNano()),
		Name:     name,
		File:     s.relative(file),
		Created:  time.Now(),
		Duration: duration,
	}
	s.recs = append(s.recs, r)
	s.saveLocked()
	return s.external(*r)
}

// rename changes a recording's display name.
func (s *Store) Rename(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil && name != "" {
		r.Name = name
		s.saveLocked()
	}
}

// setTranscript stores the ASR result for a recording and marks any derived
// summary or mind-map stale, since they were built from the old transcript.
func (s *Store) SetTranscript(id string, t *Transcript) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Transcript = t
		if r.Summary != nil {
			r.Summary.Stale = true
		}
		if r.Mindmap != nil {
			r.Mindmap.Stale = true
		}
		s.saveLocked()
	}
}

// remove drops a recording from the index; the audio file is deleted by the
// caller.
func (s *Store) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.recs {
		if r.ID == id {
			s.recs = append(s.recs[:i], s.recs[i+1:]...)
			s.saveLocked()
			return
		}
	}
}

// DropAudio forgets a recording's audio, keeping everything else it holds.
// The file itself is deleted by the caller; this only stops the entry
// pointing at it, and records that it was dropped on purpose.
func (s *Store) DropAudio(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.File, r.AudioDeleted = "", true
		s.saveLocked()
	}
}

// find returns the recording with id, or nil. The caller holds mu.
func (s *Store) find(id string) *Recording {
	for _, r := range s.recs {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// setSummary stores the generated summary for a recording.
func (s *Store) SetSummary(id string, sm *Summary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Summary = sm
		s.saveLocked()
	}
}

// setSummaryPrompt stores the editable summary-generation instruction.
func (s *Store) SetSummaryPrompt(id, prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.SummaryPrompt = prompt
		s.saveLocked()
	}
}

// setMindmap stores the generated mind-map outline.
func (s *Store) SetMindmap(id string, m *Mindmap) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Mindmap = m
		s.saveLocked()
	}
}

// setNotes stores the free-form note for a recording.
func (s *Store) SetNotes(id, notes string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Notes = notes
		s.saveLocked()
	}
}

// setSegmentText replaces one transcript segment's text (edited in the UI).
func (s *Store) SetSegmentText(id string, idx int, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(id)
	if r == nil || r.Transcript == nil || idx < 0 || idx >= len(r.Transcript.Segments) {
		return
	}
	r.Transcript.Segments[idx].Text = text
	s.saveLocked()
}

// renameSpeaker renames the speaker of segment idx. When global, every segment
// currently sharing that speaker label is renamed too.
func (s *Store) RenameSpeaker(id string, idx int, name string, global bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(id)
	if r == nil || r.Transcript == nil || idx < 0 || idx >= len(r.Transcript.Segments) {
		return
	}
	if name = strings.TrimSpace(name); name == "" {
		return
	}
	segs := r.Transcript.Segments
	if global {
		old := segs[idx].Speaker
		for i := range segs {
			if segs[i].Speaker == old {
				segs[i].Speaker = name
			}
		}
	} else {
		segs[idx].Speaker = name
	}
	s.saveLocked()
}

// appendChat adds one Q&A turn and persists it.
func (s *Store) AppendChat(id string, m ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Chat = append(r.Chat, m)
		s.saveLocked()
	}
}

// streamLastAssistant replaces the last assistant turn's text in memory only
// (no disk write) so a streaming answer can refresh the UI token by token.
func (s *Store) StreamLastAssistant(id, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(id)
	if r == nil {
		return
	}
	for i := len(r.Chat) - 1; i >= 0; i-- {
		if r.Chat[i].Role == "assistant" {
			r.Chat[i].Text = text
			return
		}
	}
}

// clearChat drops the Q&A history for a recording.
func (s *Store) ClearChat(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Chat = nil
		s.saveLocked()
	}
}

// FinishLastAssistant sets the last answer's final text and persists the
// chat; failed marks it as an error, which keeps the turn out of the context.
func (s *Store) FinishLastAssistant(id, text string, failed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(id)
	if r == nil {
		return
	}
	for i := len(r.Chat) - 1; i >= 0; i-- {
		if r.Chat[i].Role == "assistant" {
			r.Chat[i].Text, r.Chat[i].Error = text, failed
			break
		}
	}
	s.saveLocked()
}

// NewChatTopic appends a 新话题 divider, unless the chat is empty or already
// ends with one.
func (s *Store) NewChatTopic(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(id)
	if r == nil || len(r.Chat) == 0 || r.Chat[len(r.Chat)-1].Role == ChatDivider {
		return
	}
	r.Chat = append(r.Chat, ChatMessage{Role: ChatDivider})
	s.saveLocked()
}

// DeleteChat removes the message at i. A question goes with the answer
// right after it, and an answer with the question right before it, so the
// chat never holds half a turn; a divider goes alone. Dividers left at the
// start, at the end or next to each other are dropped too.
func (s *Store) DeleteChat(id string, i int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(id)
	if r == nil || i < 0 || i >= len(r.Chat) {
		return
	}
	from, to := i, i+1
	switch r.Chat[i].Role {
	case "user":
		if to < len(r.Chat) && r.Chat[to].Role == "assistant" {
			to++
		}
	case "assistant":
		if from > 0 && r.Chat[from-1].Role == "user" {
			from--
		}
	}
	r.Chat = tidyDividers(append(r.Chat[:from:from], r.Chat[to:]...))
	s.saveLocked()
}

// tidyDividers drops dividers that separate nothing.
func tidyDividers(chat []ChatMessage) []ChatMessage {
	var out []ChatMessage
	for _, m := range chat {
		if m.Role == ChatDivider && (len(out) == 0 || out[len(out)-1].Role == ChatDivider) {
			continue
		}
		out = append(out, m)
	}
	if n := len(out); n > 0 && out[n-1].Role == ChatDivider {
		out = out[:n-1]
	}
	return out
}

// ArchiveChat moves the current conversation into the archive, leaving an
// empty one. It does nothing when there is nothing to keep.
func (s *Store) ArchiveChat(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil && archiveLocked(r) {
		s.saveLocked()
	}
}

func archiveLocked(r *Recording) bool {
	chat := tidyDividers(r.Chat)
	if len(chat) == 0 {
		r.Chat = nil
		return false
	}
	title := "对话"
	for _, m := range chat {
		if m.Role == "user" && strings.TrimSpace(m.Text) != "" {
			title = cutTitle(m.Text, 30)
			break
		}
	}
	sess := ChatSession{
		ID:       fmt.Sprintf("chat-%d", time.Now().UnixNano()),
		Title:    title,
		Archived: time.Now(),
		Messages: chat,
	}
	r.ChatArchive = append([]ChatSession{sess}, r.ChatArchive...)
	r.Chat = nil
	return true
}

// cutTitle is the first line of s, at most n runes.
func cutTitle(s string, n int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// RestoreChat makes an archived conversation the current one again. The
// current one, if it holds anything, is archived in its place so nothing is
// lost by switching.
func (s *Store) RestoreChat(id, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(id)
	if r == nil {
		return
	}
	i := sessionIndex(r, sessionID)
	if i < 0 {
		return
	}
	sess := r.ChatArchive[i]
	r.ChatArchive = append(r.ChatArchive[:i:i], r.ChatArchive[i+1:]...)
	archiveLocked(r)
	r.Chat = sess.Messages
	s.saveLocked()
}

// DeleteChatSession drops an archived conversation.
func (s *Store) DeleteChatSession(id, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(id)
	if r == nil {
		return
	}
	if i := sessionIndex(r, sessionID); i >= 0 {
		r.ChatArchive = append(r.ChatArchive[:i:i], r.ChatArchive[i+1:]...)
		s.saveLocked()
	}
}

func sessionIndex(r *Recording, sessionID string) int {
	for i, sess := range r.ChatArchive {
		if sess.ID == sessionID {
			return i
		}
	}
	return -1
}

// ResetLastAnswer empties the last answer for it to be generated again,
// in memory only: the stream that follows persists it when it finishes.
func (s *Store) ResetLastAnswer(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(id)
	n := 0
	if r != nil {
		n = len(r.Chat)
	}
	if n < 2 || r.Chat[n-1].Role != "assistant" || r.Chat[n-2].Role != "user" {
		return false
	}
	r.Chat[n-1] = ChatMessage{Role: "assistant"}
	return true
}

// saveLocked writes the index atomically (tmp + rename). The caller holds mu.
func (s *Store) saveLocked() {
	data, err := json.MarshalIndent(s.recs, "", "  ")
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	os.Rename(tmp, s.path)
}
