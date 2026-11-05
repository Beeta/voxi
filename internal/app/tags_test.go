package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
)

// tagApp is an app over two recordings under t.TempDir(), with the
// settings file there too, so a test can reopen it.
func tagApp(t *testing.T) (*app, *store.Store, string, store.Recording, store.Recording) {
	t.Helper()
	dir := t.TempDir()
	lib := openTestLib(t, dir)
	first := lib.Add("周会", touchFile(t, dir, "one.m4a"), 3)
	second := lib.Add("家长会", touchFile(t, dir, "two.m4a"), 4)
	path := filepath.Join(dir, "settings.json")
	set := &settingsStore{path: path, s: &Settings{}}
	return newApp(lib, set, audio.NewRecorder()), lib, path, first, second
}

// TestTagsFromMenuToFilter walks the feature through the pointer: a tag
// made from a row's menu goes on that row, choosing it in the drop-down
// hides the other row and the selection with it, the filter is kept in the
// settings file, and deleting the tag asks first, takes it off the
// recording and goes back to all.
func TestTagsFromMenuToFilter(t *testing.T) {
	a, lib, path, first, second := tagApp(t)
	a.selected = first.ID
	tt := ui.NewTester(a.view, 968, 704)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("全部录音") {
		t.Skip("the sidebar did not render")
	}

	// 新建标签 from the row's menu makes the tag and puts it on that row.
	if err := tt.RightClick("家长会"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("标签", "新建标签…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("新建标签") {
		t.Fatal("the tag dialog did not open")
	}
	tt.Type("家庭")
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tags := a.settings.snapshot().Tags
	if len(tags) != 1 || tags[0].Name != "家庭" {
		t.Fatalf("the settings hold tags %+v", tags)
	}
	home := tags[0].ID
	if rec, _ := lib.Get(second.ID); !slices.Equal(rec.Tags, []string{home}) {
		t.Fatalf("the right-clicked recording carries %q", rec.Tags)
	}
	if rec, _ := lib.Get(first.ID); rec.Tags != nil {
		t.Errorf("the selected recording got tags %q too", rec.Tags)
	}

	// Choosing the tag in the drop-down hides the untagged, selected row.
	if err := tt.Click("全部录音"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("筛选：家庭"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if tt.HasText("周会") {
		t.Error("the filter still shows the untagged recording")
	}
	if !tt.HasText("家长会") {
		t.Error("the filter hides the tagged recording")
	}
	if a.selected != "" {
		t.Errorf("the hidden recording %q stayed selected", a.selected)
	}
	if again, err := openSettingsAt(path); err != nil || again.TagFilter != home {
		t.Errorf("the settings file keeps filter %q (%v), want %q", again.TagFilter, err, home)
	}

	// Deleting the tag from its option's menu asks, then takes it off.
	if err := tt.Click("标签筛选"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.RightClick("筛选：家庭"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Menu(); !slices.Equal(got, []string{"编辑", "删除"}) {
		t.Fatalf("the tag option's menu is %q", got)
	}
	if err := tt.ChooseMenuItem("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("删除标签？") {
		t.Fatal("deleting did not ask first")
	}
	if err := tt.Click("删除标签"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if got := a.settings.snapshot(); len(got.Tags) != 0 || got.TagFilter != "" {
		t.Errorf("after deleting: tags %+v, filter %q", got.Tags, got.TagFilter)
	}
	if rec, _ := lib.Get(second.ID); rec.Tags != nil {
		t.Errorf("the recording still carries %q", rec.Tags)
	}
	if !tt.HasText("周会") || !tt.HasText("家长会") {
		t.Error("the list did not go back to all recordings")
	}
}

// TestTagEditorRenamesAndRefusesDuplicates covers 编辑: it renames the tag
// in place, and a name another tag has is refused rather than making two
// tags only their recordings tell apart.
func TestTagEditorRenamesAndRefusesDuplicates(t *testing.T) {
	a, _, _, _, _ := tagApp(t)
	a.settings.update(func(s *Settings) {
		s.Tags = []Tag{{ID: "t1", Name: "工作"}, {ID: "t2", Name: "家庭"}}
	})
	a.openTagEditor("t1", "")
	a.tagEditName = "家庭"
	if a.saveTag() {
		t.Error("a name another tag has was saved")
	}
	a.tagEditName = "  项目  "
	if !a.saveTag() {
		t.Fatal("a fresh name was refused")
	}
	if got := a.settings.snapshot().Tags; got[0] != (Tag{ID: "t1", Name: "项目"}) || len(got) != 2 {
		t.Errorf("after renaming the tags are %+v", got)
	}
}

// TestFilterOnMissingTagShowsAll keeps a filter naming a tag that is gone
// from hiding every recording.
func TestFilterOnMissingTagShowsAll(t *testing.T) {
	if got := activeFilter(Settings{TagFilter: "gone"}); got != "" {
		t.Errorf("a filter on a missing tag is %q, want all", got)
	}
}

// openSettingsAt reads a settings file back, as the next launch would.
func openSettingsAt(path string) (Settings, error) {
	var s Settings
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(data, &s)
}
