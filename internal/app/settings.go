package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/zhaorongcun/voxi/internal/tunnel"
)

// Settings is the app's persisted configuration: which sources to capture,
// where the recordings go, the ASR key, and the single OpenAI-compatible LLM
// endpoint.
type Settings struct {
	Mic             bool   `json:"mic"`             // capture the microphone
	System          bool   `json:"system"`          // capture the system output
	RecordingsDir   string `json:"recordingsDir"`   // where the .m4a files go; empty = the downloads directory
	ASRKey          string `json:"asrKey"`          // ASR api key
	CloudflaredPath string `json:"cloudflaredPath"` // path to the cloudflared binary
	LLMBaseURL      string `json:"llmBaseURL"`      // OpenAI-compatible chat/completions endpoint
	LLMKey          string `json:"llmKey"`          // LLM api key
	LLMModel        string `json:"llmModel"`        // model id
}

// recordingsDir returns where new recordings are written: the configured
// directory, or the downloads directory when it is not set (which is also
// where recordings went before the directory was configurable).
func recordingsDir(s Settings) string {
	if d := strings.TrimSpace(s.RecordingsDir); d != "" {
		return d
	}
	if d, err := mygo.App.Path(mygo.PathDownloads); err == nil && d != "" {
		return d
	}
	d, _ := mygo.App.Path(mygo.PathHome)
	return d
}

// recordingsDir is recordingsDir for the current settings.
func (a *app) recordingsDir() string {
	return recordingsDir(a.settings.snapshot())
}

// settingsStore holds the configuration and its JSON file. It is safe for
// concurrent use: the UI edits it while a future ASR/LLM goroutine reads a
// snapshot.
type settingsStore struct {
	mu   sync.Mutex
	path string
	s    *Settings
}

// openSettings loads settings.json from the app data directory, returning
// sensible defaults (both sources on) when the file does not exist yet.
func openSettings() (*settingsStore, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	st := &settingsStore{
		path: filepath.Join(dir, "settings.json"),
		s:    &Settings{Mic: true, System: true, CloudflaredPath: tunnel.DefaultCloudflaredPath},
	}
	data, err := os.ReadFile(st.path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, st.s); err != nil {
		return nil, err
	}
	return st, nil
}

// snapshot returns a copy so the UI can render (and bind edits to locals)
// without holding the lock.
func (st *settingsStore) snapshot() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return *st.s
}

// update applies fn to the settings under the lock and persists the result.
// Every edit goes through it.
func (st *settingsStore) update(fn func(*Settings)) {
	st.mu.Lock()
	defer st.mu.Unlock()
	fn(st.s)
	st.saveLocked()
}

// saveLocked writes the file atomically (tmp + rename). The caller holds mu.
func (st *settingsStore) saveLocked() {
	data, err := json.MarshalIndent(st.s, "", "  ")
	if err != nil {
		return
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	os.Rename(tmp, st.path)
}
