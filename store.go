package main

import (
	"encoding/json"
	"fmt"
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

// ChatMessage is one turn of the transcript Q&A. Role is "user" or "assistant".
type ChatMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// Recording is one captured file and what the library knows about it.
type Recording struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	File          string        `json:"file"` // absolute path to the .m4a
	Created       time.Time     `json:"created"`
	Duration      float64       `json:"duration"` // seconds
	Transcript    *Transcript   `json:"transcript,omitempty"`
	Summary       *Summary      `json:"summary,omitempty"`
	SummaryPrompt string        `json:"summaryPrompt,omitempty"` // editable instruction; empty = default
	Mindmap       *Mindmap      `json:"mindmap,omitempty"`
	Chat          []ChatMessage `json:"chat,omitempty"`
	Notes         string        `json:"notes,omitempty"`
}

// store is the recording library and its JSON index on disk. It is safe for
// concurrent use: the UI reads snapshots while capture and ASR goroutines
// write.
type store struct {
	mu   sync.Mutex
	path string
	recs []*Recording
}

// openStore loads the library index from the app's data directory, returning
// an empty library when the file does not exist yet.
func openStore() (*store, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	s := &store{path: filepath.Join(dir, "recordings.json")}
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

// snapshot returns a copy of the recordings, newest first, so the UI can
// render without holding the lock.
func (s *store) snapshot() []Recording {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Recording, len(s.recs))
	for i, r := range s.recs {
		out[i] = *r
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

// get returns a copy of one recording.
func (s *store) get(id string) (Recording, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		return *r, true
	}
	return Recording{}, false
}

// add registers a newly captured file and returns its record.
func (s *store) add(name, file string, duration float64) Recording {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := &Recording{
		ID:       fmt.Sprintf("rec-%d", time.Now().UnixNano()),
		Name:     name,
		File:     file,
		Created:  time.Now(),
		Duration: duration,
	}
	s.recs = append(s.recs, r)
	s.saveLocked()
	return *r
}

// rename changes a recording's display name.
func (s *store) rename(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil && name != "" {
		r.Name = name
		s.saveLocked()
	}
}

// setTranscript stores the ASR result for a recording and marks any derived
// summary or mind-map stale, since they were built from the old transcript.
func (s *store) setTranscript(id string, t *Transcript) {
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
func (s *store) remove(id string) {
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

// find returns the recording with id, or nil. The caller holds mu.
func (s *store) find(id string) *Recording {
	for _, r := range s.recs {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// setSummary stores the generated summary for a recording.
func (s *store) setSummary(id string, sm *Summary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Summary = sm
		s.saveLocked()
	}
}

// setSummaryPrompt stores the editable summary-generation instruction.
func (s *store) setSummaryPrompt(id, prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.SummaryPrompt = prompt
		s.saveLocked()
	}
}

// setMindmap stores the generated mind-map outline.
func (s *store) setMindmap(id string, m *Mindmap) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Mindmap = m
		s.saveLocked()
	}
}

// setNotes stores the free-form note for a recording.
func (s *store) setNotes(id, notes string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Notes = notes
		s.saveLocked()
	}
}

// setSegmentText replaces one transcript segment's text (edited in the UI).
func (s *store) setSegmentText(id string, idx int, text string) {
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
func (s *store) renameSpeaker(id string, idx int, name string, global bool) {
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
func (s *store) appendChat(id string, m ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Chat = append(r.Chat, m)
		s.saveLocked()
	}
}

// streamLastAssistant replaces the last assistant turn's text in memory only
// (no disk write) so a streaming answer can refresh the UI token by token.
func (s *store) streamLastAssistant(id, text string) {
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

// commitChat persists the chat after a streamed answer completes.
func (s *store) commitChat(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.find(id) != nil {
		s.saveLocked()
	}
}

// clearChat drops the Q&A history for a recording.
func (s *store) clearChat(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.find(id); r != nil {
		r.Chat = nil
		s.saveLocked()
	}
}

// saveLocked writes the index atomically (tmp + rename). The caller holds mu.
func (s *store) saveLocked() {
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
