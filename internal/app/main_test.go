package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/Beeta/voxi/internal/store"
)

// TestMain points the app's data directory at a fresh temporary one for the
// whole run, so nothing a test does — directly or through the framework —
// lands in ~/Library/Application Support. Tests that need a library still
// open their own with openTestLib, so they never see each other's
// recordings either.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "voxi-test-")
	if err != nil {
		panic(err)
	}
	mygo.App.SetPath(mygo.PathUserData, dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// openTestLib is an empty library whose index and recordings live under
// dir, normally t.TempDir().
func openTestLib(t *testing.T, dir string) *store.Store {
	t.Helper()
	lib, err := store.Open(filepath.Join(dir, "recordings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	return lib
}
