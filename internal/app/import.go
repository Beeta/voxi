package app

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
)

// importExts are the formats both halves of the app accept. The player opens
// more than this — flac, aiff, opus and caf all played back in a probe — but
// asr.detectFormat knows only the service's own list and would submit
// anything else as m4a, which the service would then parse as the wrong
// container. Keep the two lists in step when either side grows.
var importExts = []string{"m4a", "mp3", "wav", "aac", "ogg", "amr"}

// importable reports whether path carries one of those extensions. The
// chooser filters on them too, but a typed-in name or a renamed file can
// still arrive without one.
func importable(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	for _, ok := range importExts {
		if ext == ok {
			return true
		}
	}
	return false
}

// importName is the library name for an imported file: its own name without
// the extension, or a timestamp when that leaves nothing to show.
func importName(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" {
		return "导入 " + time.Now().Format("01-02 15:04")
	}
	return name
}

// importButton is the sidebar header's way into importing an audio file.
func (a *app) importButton(c *ui.Context) {
	a.mu.Lock()
	busy := a.importing
	a.mu.Unlock()
	btn := ui.ButtonBase(c.Key("import")).Padding(6).Radius(8).Center().Disabled(busy).Tooltip("导入音频文件")
	btn.Background(controlFill(btn, cardBg2, cardBg3))
	btn.Children(func() { ui.Icon(c, iconImport).FontSize(16).TextColor(muted) })
	if btn.Clicked() {
		a.importAudio()
	}
}

// importAudio asks for audio files and adds them to the library. The chooser
// runs off the UI thread: mygo's dialogs post to the main thread and then
// wait for it, which would deadlock if the call came from the main thread
// itself.
func (a *app) importAudio() {
	a.mu.Lock()
	if a.importing {
		a.mu.Unlock()
		return
	}
	a.importing = true
	a.mu.Unlock()
	a.ping()

	go func() {
		defer func() {
			a.mu.Lock()
			a.importing = false
			a.mu.Unlock()
			a.ping()
		}()
		files, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:       "导入音频",
			ButtonLabel: "导入",
			Message:     "支持 " + strings.Join(importExts, "、"),
			Filters:     []mygo.FileFilter{{Name: "音频文件", Extensions: importExts}},
			Multiple:    true,
		})
		if err != nil || len(files) == 0 {
			return
		}
		a.addImported(files)
	}()
}

// addImported copies each file into the library and registers it, then selects
// the last one. A file that cannot be imported is collected rather than
// aborting the batch, and reported in a dialog at the end: silence would
// leave the user counting rows to find out which ones made it.
func (a *app) addImported(files []string) {
	var failures []string
	selected := ""
	for _, src := range files {
		base := filepath.Base(src)
		if !importable(src) {
			failures = append(failures, base+"：不支持的格式")
			continue
		}
		// The player validates the file and measures it at once: it is what
		// will play the file back, and its duration is what the seek bar and
		// the transcript's fallback segment need.
		p, err := audio.NewPlayer(src)
		if err != nil {
			failures = append(failures, base+"：无法打开，文件可能已损坏")
			continue
		}
		dur := p.Duration()
		p.Close()
		if dur <= 0 {
			failures = append(failures, base+"：读不到时长")
			continue
		}
		dst, err := a.lib.ImportFile(src)
		if err != nil {
			failures = append(failures, base+"："+err.Error())
			continue
		}
		selected = a.lib.Add(importName(src), dst, dur).ID
	}
	if selected != "" {
		a.mu.Lock()
		a.selected = selected
		a.mu.Unlock()
	}
	if len(failures) > 0 {
		a.mu.Lock()
		a.importFailMsg, a.importFailOpen = strings.Join(failures, "\n"), true
		a.mu.Unlock()
	}
	a.ping()
}

// importFailureDialog reports what an import could not take.
func (a *app) importFailureDialog(c *ui.Context) {
	if i := ui.AlertDialog(c.Key("import-fail"), &a.importFailOpen, "以下文件未能导入", a.importFailMsg, "好"); i >= 0 {
		a.importFailMsg = ""
	}
}
