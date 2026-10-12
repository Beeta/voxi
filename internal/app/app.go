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
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
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
	pickingDir     bool // a recordings-directory dialog is open

	// The recording row's context menu: which recording it is renaming or
	// deleting, and the rename draft. A row can be right-clicked without
	// being selected, so these name the target rather than reading the
	// selection.
	renameRecOpen bool
	renameRecID   string
	renameRecName string
	delRecID      string

	// Importing audio files: a chooser or copies are in flight, and what
	// could not be taken, reported once the batch is done.
	importing      bool
	importFailOpen bool
	importFailMsg  string

	// mdCache holds parsed Markdown, keyed by the text it was parsed from,
	// so a summary is not reparsed on every frame.
	mdCache map[string]mdEntry

	notify func()

	// setTheme hands the appearance to the framework, which repaints every
	// window with it. A field rather than a direct call so a test can watch
	// the choice: the real call must run on the main thread, and a test
	// process has no event loop to run it on.
	setTheme func(mygo.ThemeSource)

	// version is the app's version, shown in the settings dialog. It is
	// empty for a build that carries none, such as a plain `go run`.
	version string
}

func newApp(st *store.Store, set *settingsStore, rec *audio.Recorder) *app {
	a := &app{
		lib:          st,
		settings:     set,
		rec:          rec,
		version:      mygo.App.Version(),
		transcribing: map[string]bool{},
		asrErr:       map[string]string{},
		summarizing:  map[string]bool{},
		mindmapping:  map[string]bool{},
		asking:       map[string]bool{},
		expanded:     map[string]bool{},
		draftQ:       map[string]string{},
		draftPrompt:  map[string]string{},
		promptOpen:   map[string]bool{},
		mdCache:      map[string]mdEntry{},
		editSeg:      -1,
		setTheme:     mygo.Theme.SetSource,
	}
	if recs := st.Snapshot(); len(recs) > 0 {
		a.selected = recs[0].ID
	}
	return a
}

// Run opens the library and the settings, then runs the app until its window
// closes. It returns the error from mygo.App.Run.
func Run() error {
	set, err := openSettings()
	if err != nil {
		return err
	}
	st, err := store.OpenStore(recordingsDir(set.snapshot()))
	if err != nil {
		return err
	}
	a := newApp(st, set, audio.NewRecorder())
	// The saved appearance, applied before the first frame so the window
	// never opens in the wrong one. The views follow it through
	// Context.Theme; this only tells the backend and the window chrome.
	mygo.Theme.SetSource(appearanceSource(set.snapshot().Appearance))
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: "Voxi",
			// 968x704 is 880x640 grown by 10%; the settings dialog in
			// settingsview.go scaled by the same factor.
			Width:     968,
			Height:    704,
			MinWidth:  720,
			MinHeight: 480,
			// The window's own color, seen for the instant before the first
			// frame paints; the app draws over it after that. Both
			// appearances, so that instant is not a flash of the other one.
			BackgroundColor: "light-dark(#f2f2f7, #1c1c1e)",
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
