// Voxi (有听有记) is a MyGo app for capturing meeting audio and turning it
// into usable notes: record the microphone and the system output into an AAC
// (.m4a) file, then browse the library, play a recording back on a seekable
// timeline, transcribe it to timed text through an ASR service, and build
// summaries, mind maps, Q&A and notes on top of the transcript.
//
//	go run .
//
// Capturing and playback are macOS only (ScreenCaptureKit + AVAudioPlayer); on
// other platforms the window still opens and explains that.
package main

import (
	"log"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
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
	store    *store
	settings *settingsStore
	rec      *recorder

	mu           sync.Mutex
	selected     string
	player       *player
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

func newApp(st *store, set *settingsStore, rec *recorder) *app {
	a := &app{
		store:        st,
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
	if recs := st.snapshot(); len(recs) > 0 {
		a.selected = recs[0].ID
	}
	return a
}

func main() {
	st, err := openStore()
	if err != nil {
		log.Fatal(err)
	}
	set, err := openSettings()
	if err != nil {
		log.Fatal(err)
	}
	a := newApp(st, set, newRecorder())
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
		a.rec.mu.Lock()
		a.rec.notify = update
		a.rec.mu.Unlock()
		go a.tick()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
