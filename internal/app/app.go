// Package app is Voxi's (有听有记) interface and state: the recording
// library, the selected recording and its player, and the record, transcribe,
// summarize, mind-map, Q&A and notes actions behind them.
//
// Run opens the app; cmd/voxi calls it.
package app

import (
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/zhaorongcun/voxi/internal/audio"
	"github.com/zhaorongcun/voxi/internal/store"
)

// The palette.
var (
	pageBg  = ui.Hex("#1c1c1e")
	cardBg  = ui.Hex("#2c2c2e")
	line    = ui.Hex("#3a3a3c")
	ink     = ui.Hex("#f5f5f7")
	muted   = ui.Hex("#9a9a9e")
	accent  = ui.Hex("#0a84ff")
	recRed  = ui.Hex("#ff453a")
	okGreen = ui.Hex("#30d158")
	cardBg2 = ui.Hex("#333335")
)

// app is the whole UI: the recording library, the current selection and its
// player, plus transient record/transcribe state. mu guards the fields that
// background goroutines (capture + ASR) touch while the UI thread reads them.
type app struct {
	lib      *store.Store
	settings *settingsStore
	rec      *audio.Recorder

	mu           sync.Mutex
	selected     string
	player       *audio.Player
	playerID     string
	playPos      float64
	transcribing map[string]bool
	asrErr       map[string]string
	summarizing  map[string]bool
	mindmapping  map[string]bool
	asking       map[string]bool
	status       string
	verifyingLLM bool
	llmVerifyMsg string
	llmVerifyOK  bool

	recording bool

	// UI-thread-only view state (no lock needed).
	tab            int
	expanded       map[string]bool   // mind-map node collapse state, keyed by path
	draftQ         map[string]string // per-recording Q&A input draft
	draftPrompt    map[string]string // per-recording summary-prompt edit draft
	promptOpen     map[string]bool   // per-recording: is the summary prompt expanded
	editSeg        int               // transcript segment index being edited, -1 = none
	renaming       bool              // speaker-rename modal open
	renameIdx      int               // segment whose speaker is being renamed
	renameDraft    string            // speaker name being typed
	confirmRetrans bool              // re-transcribe confirmation open
	showSettings   bool
	confirmDel     bool

	notify func()
}

func newApp(st *store.Store, set *settingsStore, rec *audio.Recorder) *app {
	a := &app{
		lib:          st,
		settings:     set,
		rec:          rec,
		transcribing: map[string]bool{},
		asrErr:       map[string]string{},
		summarizing:  map[string]bool{},
		mindmapping:  map[string]bool{},
		asking:       map[string]bool{},
		expanded:     map[string]bool{},
		draftQ:       map[string]string{},
		draftPrompt:  map[string]string{},
		promptOpen:   map[string]bool{},
		editSeg:      -1,
	}
	if recs := st.Snapshot(); len(recs) > 0 {
		a.selected = recs[0].ID
	}
	return a
}

// Run opens the library and the settings, then runs the app until its window
// closes. It returns the error from mygo.App.Run.
func Run() error {
	st, err := store.OpenStore()
	if err != nil {
		return err
	}
	set, err := openSettings()
	if err != nil {
		return err
	}
	a := newApp(st, set, audio.NewRecorder())
	mygo.Theme.SetSource(mygo.ThemeDark)
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "Voxi",
			Width:           880,
			Height:          640,
			MinWidth:        720,
			MinHeight:       480,
			BackgroundColor: "#1c1c1e",
			Content:         ui.View(a.view),
		})
		update := func() { win.Update(func() {}) }
		a.mu.Lock()
		a.notify = update
		a.mu.Unlock()
		a.rec.SetNotify(update)
		go a.tick()
	})
	return mygo.App.Run()
}
