package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
)

// waitRec frames tt until the recorder reaches want.
func waitRec(t *testing.T, a *app, tt *ui.Tester, want audio.State) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if st, _, _, _ := a.rec.Snapshot(); st == want {
			tt.Frame()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	st, _, _, _ := a.rec.Snapshot()
	t.Fatalf("the recorder is in state %d, want %d", st, want)
}

// TestPauseAndResumeFromTheControl drives the sidebar's record control:
// 暂停 pauses and says so, 继续 resumes, and 停止并保存 works from a pause.
func TestPauseAndResumeFromTheControl(t *testing.T) {
	dir := t.TempDir()
	set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &Settings{Mic: true, RecordingsDir: dir}}
	a := newApp(openTestLib(t, dir), set, audio.NewSimulatedRecorder())
	tt := ui.NewTester(a.recordControl, 300, 300)
	tt.Frame()
	if err := tt.Click("开始录制"); err != nil {
		t.Fatal(err)
	}
	waitRec(t, a, tt, audio.StateRecording)
	if !tt.HasText("录制中") {
		t.Fatalf("recording shows %q", tt.Texts())
	}

	if err := tt.Click("暂停"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if st, _, _, _ := a.rec.Snapshot(); st != audio.StatePaused {
		t.Fatalf("暂停 left the recorder in state %d", st)
	}
	if !tt.HasText("已暂停") || !tt.HasText("继续") {
		t.Errorf("paused shows %q", tt.Texts())
	}

	if err := tt.Click("继续"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if st, _, _, _ := a.rec.Snapshot(); st != audio.StateRecording {
		t.Fatalf("继续 left the recorder in state %d", st)
	}

	tt.Click("暂停")
	tt.Frame()
	if err := tt.Click("停止并保存"); err != nil {
		t.Fatal(err)
	}
	waitRec(t, a, tt, audio.StateIdle)
	if tt.HasText("已暂停") {
		t.Error("the control still says paused after stopping")
	}
}

// TestLowDiskWarning: a disk with room gets no warning; a nearly full one
// gets a note naming the risk. The second case is exercised where it can
// be, by handing lowDiskWarning a threshold the disk is under.
func TestLowDiskWarning(t *testing.T) {
	dir := t.TempDir()
	free, err := freeDisk(dir)
	if err != nil {
		t.Skip("no free-space check here")
	}
	if free >= diskFloor && lowDiskWarning(dir) != "" {
		t.Error("a disk with room still warns")
	}
	old := diskFloor
	diskFloor = free * 2 // as if almost no room were left
	defer func() { diskFloor = old }()
	if w := lowDiskWarning(dir); !strings.Contains(w, "磁盘仅剩") || !strings.Contains(w, "自动看护") {
		t.Errorf("the warning is %q", w)
	}
}
