// Package app is Voxi's (有听有记) interface and state: the recording
// library, the selected recording and its player, and the record, transcribe,
// summarize, mind-map, Q&A and notes actions behind them.
//
// Run opens the app; cmd/voxi calls it.
package app

import (
	"context"
	"sync"

	"github.com/Beeta/voxi/internal/asr"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
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
	asrProgress  map[string]asr.Progress // the steps of a transcription under way, or of the last that failed
	summarizing  map[string]bool
	mindmapping  map[string]bool
	asking       map[string]bool
	askCancel    map[string]context.CancelFunc // stops the answer streaming for a recording
	status       string
	verifyingLLM bool   // a verification of llmVerifyID is under way
	llmVerifyID  string // the provider the message below is about
	llmVerifyMsg string
	llmVerifyOK  bool
	// Model lists fetched from each provider's /models, by provider id.
	llmFetching map[string]bool
	llmFetched  map[string][]string
	llmFetchErr map[string]string

	recording bool

	// UI-thread-only view state (no lock needed).
	tab             int
	expanded        map[string]bool   // mind-map node collapse state, keyed by path
	draftQ          map[string]string // per-recording Q&A input draft
	draftPrompt     map[string]string // per-recording summary-prompt edit draft
	promptOpen      map[string]bool   // per-recording: is the summary prompt expanded
	editSeg         int               // transcript segment index being edited, -1 = none
	renaming        bool              // speaker-rename modal open
	renameIdx       int               // segment whose speaker is being renamed
	renameDraft     string            // speaker name being typed
	confirmRetrans  bool              // re-transcribe confirmation open
	showSettings    bool
	settingsTab     int    // the settings page shown (settingsPages)
	llmOpenProv     string // the provider expanded for editing in settings
	confirmDel      bool
	confirmDelAudio bool              // dropping the audio while keeping the text
	delProviderID   string            // the LLM provider the delete confirmation is about
	llmFilter       map[string]string // per-provider filter over its fetched models
	llmShowFetched  map[string]bool   // per-provider: is its fetched model list unfolded
	confirmDelProv  bool
	// Q&A tab: the archived-conversations view, which of them is expanded,
	// and the clear / delete-session confirmations.
	qaHistory    bool
	qaFocusInput bool // focus the Q&A input in the next frame
	qaCaretEnd   bool // put the caret at the end of the Q&A input then
	// The Q&A preset editor: open on preset presetEditIdx (-1 for a new
	// one), with its draft; and the delete confirmation.
	presetEditOpen    bool
	presetEditIdx     int
	presetEditLabel   string
	presetEditText    string
	confirmDelPreset  bool
	delPresetIdx      int
	qaOpenSession     string
	confirmClearChat  bool
	confirmDelSession bool
	delSessionID      string
	pickingDir        bool // a recordings-directory dialog is open

	// The recording row's context menu: which recording it is renaming or
	// deleting, and the rename draft. A row can be right-clicked without
	// being selected, so these name the target rather than reading the
	// selection.
	renameRecOpen bool
	renameRecID   string
	renameRecName string
	delRecID      string
	delAudioID    string

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

	// trashItem moves a file to the Trash. A field for the same reason as
	// setTheme: the real one runs on the main thread, so a test that watched
	// a whole delete would wait for an event loop that never comes.
	trashItem func(string) error

	// runASR transcribes a file, reporting each step. A field so a test can
	// drive the transcript tab through the steps without cloudflared or
	// the service.
	runASR func(key, cloudflared, path string, duration float64, onProgress func(asr.Progress)) (*store.Transcript, error)

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
		asrProgress:  map[string]asr.Progress{},
		summarizing:  map[string]bool{},
		mindmapping:  map[string]bool{},
		asking:       map[string]bool{},
		askCancel:    map[string]context.CancelFunc{},
		llmFetching:  map[string]bool{},
		llmFetched:   map[string][]string{},
		llmFetchErr:  map[string]string{},
		llmFilter:    map[string]string{},
		expanded:     map[string]bool{},
		draftQ:       map[string]string{},
		draftPrompt:  map[string]string{},
		promptOpen:   map[string]bool{},
		mdCache:      map[string]mdEntry{},
		editSeg:      -1,
		setTheme:     mygo.Theme.SetSource,
		trashItem:    mygo.Shell.TrashItem,
		runASR: func(key, cf, path string, dur float64, on func(asr.Progress)) (*store.Transcript, error) {
			return asr.New(key, cf).Transcribe(path, dur, on)
		},

		// per-provider fold state of the fetched model lists
		llmShowFetched: map[string]bool{},
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
		win := mygo.NewWindow(windowOptions(ui.View(a.view)))
		update := func() { win.Update(func() {}) }
		a.mu.Lock()
		a.notify = update
		a.mu.Unlock()
		a.rec.SetNotify(update)
		go a.tick()
	})
	return mygo.App.Run()
}

// windowOptions is the main window: what a first run opens at, and what the
// app remembers of it afterwards. Its own function so the remembering can be
// tested — a window without a state key loses the user's size silently, with
// nothing failing.
func windowOptions(content mygo.Content) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title: "Voxi",
		// 968x704 is 880x640 grown by 10%; the settings dialog in
		// settingsview.go scaled by the same factor. It is what a first run
		// opens at: once the user has sized the window, the state below
		// opens it where and how they left it.
		Width:     968,
		Height:    704,
		MinWidth:  720,
		MinHeight: 480,
		// StateKey remembers the window's position, size and maximized and
		// full screen state, in window-state.json beside the settings, and
		// gives them back at the next launch — as long as the window would
		// still show on a connected display, which is what keeps a window
		// parked on a since-unplugged monitor from opening off screen. It
		// also keeps a size the display no longer has room for from
		// opening larger than the screen.
		StateKey: "main",
		// The window's own color, seen for the instant before the first
		// frame paints; the app draws over it after that. Both appearances,
		// so that instant is not a flash of the other one.
		BackgroundColor: "light-dark(#f2f2f7, #1c1c1e)",
		Content:         content,
	}
}
