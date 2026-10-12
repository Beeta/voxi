package app

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
)

// TestRowContextMenu covers the recording row's context menu: the items it
// offers, that rename edits the right-clicked recording rather than the
// selection, and that delete asks before touching the file.
func TestRowContextMenu(t *testing.T) {
	dir := t.TempDir()
	lib, err := store.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := lib.Add("第一次", filepath.Join(dir, "one.m4a"), 3)
	second := lib.Add("第二次", filepath.Join(dir, "two.m4a"), 4)
	set := &settingsStore{path: filepath.Join(dir, "settings.json"), s: &Settings{}}
	a := newApp(lib, set, audio.NewRecorder())
	// The selection deliberately differs from the row the menu opens on:
	// the menu acts on the row it was opened on, not on the selection.
	a.selected = first.ID

	tt := ui.NewTester(a.view, 968, 704)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("第二次") {
		// Capture is unavailable off macOS, where the view renders a notice
		// instead of the sidebar.
		t.Skip("the sidebar did not render")
	}
	// The detail pane keeps no action bar: these actions live on the row's
	// context menu now.
	if tt.HasText("在访达中显示") {
		t.Error("the detail pane still shows the action bar")
	}

	if err := tt.RightClick("第二次"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Menu(); !slices.Equal(got, []string{"重命名", "在访达中显示", "-", "删除"}) {
		t.Fatalf("the row menu is %q", got)
	}

	// Rename opens on the right-clicked row, which is not the selected one.
	if err := tt.ChooseMenuItem("重命名"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.renameRecOpen || a.renameRecID != second.ID {
		t.Fatalf("rename opened on %q (open=%v), want the right-clicked %q",
			a.renameRecID, a.renameRecOpen, second.ID)
	}
	if !tt.HasText("重命名录音") {
		t.Error("the rename dialog did not render")
	}

	a.renameRecName = "改过的名字" // what the field holds once the user types
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if a.renameRecOpen {
		t.Error("saving left the rename dialog open")
	}
	if rec, _ := lib.Get(second.ID); rec.Name != "改过的名字" {
		t.Errorf("the right-clicked recording is named %q, want %q", rec.Name, "改过的名字")
	}
	if rec, _ := lib.Get(first.ID); rec.Name != "第一次" {
		t.Errorf("renaming one row renamed the selected one to %q too", rec.Name)
	}

	// Delete confirms first: the file is only trashed once the user says so.
	tt.Frame()
	if err := tt.RightClick("改过的名字"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.confirmDel || a.delRecID != second.ID {
		t.Fatalf("delete asked about %q (open=%v), want the right-clicked %q",
			a.delRecID, a.confirmDel, second.ID)
	}
	if !tt.HasText("将把它移到废纸篓") {
		t.Error("the delete confirmation did not render")
	}
}
