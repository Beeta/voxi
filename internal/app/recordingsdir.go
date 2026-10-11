package app

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/zhaorongcun/voxi/internal/store"
)

// recordingsDirRow shows where new recordings are written and lets the user
// pick another directory.
func (a *app) recordingsDirRow(c *ui.Context, s Settings) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "录音存放位置").FontSize(14).TextColor(ink)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, recordingsDir(s)).FontSize(12).TextColor(muted).Grow(1)
			btn := ui.ButtonBase(c.Key("rec-dir")).Padding(6, 12).Radius(8).Center().Background(cardBg2).Border(1, line)
			btn.Children(func() { ui.Text(c, "选择…").FontSize(13).TextColor(ink) })
			if btn.Clicked() {
				go a.chooseRecordingsDir()
			}
		})
		ui.Text(c, "改到别的目录时可以选择把已有录音一起搬过去；不搬的话旧录音留在原处，仍可播放和识别。").FontSize(12).TextColor(muted)
	})
}

// chooseRecordingsDir asks for a directory, checks that recordings can be
// written there, and offers to move the existing ones into it.
//
// It runs on its own goroutine, not in the render pass: mygo's dialogs post
// to the main thread and then wait for it, which would deadlock if the call
// came from the main thread itself.
func (a *app) chooseRecordingsDir() {
	a.mu.Lock()
	if a.pickingDir {
		a.mu.Unlock()
		return
	}
	a.pickingDir = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.pickingDir = false
		a.mu.Unlock()
	}()

	old := a.recordingsDir()
	dirs, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       "选择录音存放位置",
		DefaultPath: old,
		ButtonLabel: "选择",
		Directory:   true,
	})
	if err != nil || len(dirs) == 0 {
		return
	}
	dir := dirs[0]
	if store.SameDir(dir, old) {
		return
	}
	if err := checkWritableDir(dir); err != nil {
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Type:    mygo.MessageError,
			Title:   "无法使用该目录",
			Message: dir,
			Detail:  err.Error(),
		})
		return
	}

	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageQuestion,
		Title:   "已有录音要一起搬过去吗？",
		Message: "新录音将存到：" + dir,
		Detail: "移动：把已有录音搬到新目录，记录保持可用。\n" +
			"不移动：已有录音留在 " + old + "，仍可播放和识别；只有新录音写到新目录。",
		Buttons:       []string{"移动已有录音", "只改新录音位置", "取消"},
		DefaultButton: 0,
		CancelButton:  2,
	})
	if err != nil || res.Button == 2 {
		return
	}

	if res.Button == 0 {
		n, err := a.lib.MigrateTo(dir)
		if err != nil {
			_, _ = mygo.Dialog.Message(mygo.MessageOptions{
				Type:    mygo.MessageError,
				Title:   "迁移失败",
				Message: "已有录音都没有改动。",
				Detail:  err.Error(),
			})
			return
		}
		a.settings.update(func(st *Settings) { st.RecordingsDir = dir })
		a.setStatus("已移动 " + strconv.Itoa(n) + " 个录音到新目录")
		return
	}

	a.lib.SetRoot(dir)
	a.settings.update(func(st *Settings) { st.RecordingsDir = dir })
	a.setStatus("新录音将存到新目录")
}

// checkWritableDir reports why dir cannot hold recordings, or nil when it can.
// It probes with a real file: a permission bit alone does not tell the whole
// story on macOS (sandboxing, read-only volumes).
func checkWritableDir(dir string) error {
	st, err := os.Stat(dir)
	if err != nil {
		return errors.New("目录不存在或无法访问")
	}
	if !st.IsDir() {
		return errors.New("选择的不是一个目录")
	}
	probe := filepath.Join(dir, ".voxi-write-probe")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return errors.New("目录不可写")
	}
	f.Close()
	os.Remove(probe)
	return nil
}
