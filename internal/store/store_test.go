package store

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestStore returns a store whose index and recordings live in fresh temp
// directories.
func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "recordings")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return &Store{path: filepath.Join(dir, "recordings.json"), root: root}, root
}

// touch writes an empty file and returns its path.
func touch(t *testing.T, path string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAddStoresRelativePath(t *testing.T) {
	s, root := newTestStore(t)
	file := touch(t, filepath.Join(root, "a.m4a"))

	rec := s.Add("a", file, 1)
	if rec.File != file {
		t.Fatalf("Add returned %q, want the absolute path %q", rec.File, file)
	}
	if got := s.recs[0].File; got != "a.m4a" {
		t.Fatalf("stored %q, want %q", got, "a.m4a")
	}
	if got := s.Snapshot()[0].File; got != file {
		t.Fatalf("Snapshot returned %q, want %q", got, file)
	}
}

func TestAddKeepsAbsolutePathOutsideRoot(t *testing.T) {
	s, _ := newTestStore(t)
	outside := touch(t, filepath.Join(t.TempDir(), "b.m4a"))

	rec := s.Add("b", outside, 1)
	if s.recs[0].File != outside {
		t.Fatalf("stored %q, want the absolute path %q", s.recs[0].File, outside)
	}
	if rec.File != outside {
		t.Fatalf("Add returned %q, want %q", rec.File, outside)
	}
}

// TestLegacyAbsolutePathsStillResolve covers libraries written before the
// recordings directory was configurable: those files are absolute and must
// keep working untouched.
func TestLegacyAbsolutePathsStillResolve(t *testing.T) {
	s, root := newTestStore(t)
	legacy := touch(t, filepath.Join(root, "old.m4a"))
	s.recs = []*Recording{{ID: "rec-1", Name: "old", File: legacy}}

	if got := s.Snapshot()[0].File; got != legacy {
		t.Fatalf("Snapshot returned %q, want %q", got, legacy)
	}
	if got, ok := s.Get("rec-1"); !ok || got.File != legacy {
		t.Fatalf("Get returned %q (ok=%v), want %q", got.File, ok, legacy)
	}
}

func TestMigrateToMovesFilesAndIndex(t *testing.T) {
	s, root := newTestStore(t)
	file := touch(t, filepath.Join(root, "a.m4a"))
	s.Add("a", file, 1)

	dest := filepath.Join(t.TempDir(), "moved")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	n, err := s.MigrateTo(dest)
	if err != nil {
		t.Fatalf("MigrateTo: %v", err)
	}
	if n != 1 {
		t.Fatalf("moved %d files, want 1", n)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("source %q still exists", file)
	}
	want := filepath.Join(dest, "a.m4a")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("destination missing: %v", err)
	}
	if got := s.Snapshot()[0].File; got != want {
		t.Fatalf("after migration Snapshot returned %q, want %q", got, want)
	}
	if s.Root() != dest {
		t.Fatalf("root = %q, want %q", s.Root(), dest)
	}
}

func TestMigrateToLeavesOutsideFilesAlone(t *testing.T) {
	s, _ := newTestStore(t)
	outside := touch(t, filepath.Join(t.TempDir(), "b.m4a"))
	s.Add("b", outside, 1)

	n, err := s.MigrateTo(t.TempDir())
	if err != nil {
		t.Fatalf("MigrateTo: %v", err)
	}
	if n != 0 {
		t.Fatalf("moved %d files, want 0", n)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("file outside the recordings directory was moved: %v", err)
	}
	if got := s.Snapshot()[0].File; got != outside {
		t.Fatalf("Snapshot returned %q, want %q", got, outside)
	}
}

// TestMigrateToRollsBackOnFailure makes the second move fail and checks the
// first one was put back, so a failed migration leaves the library as it was.
func TestMigrateToRollsBackOnFailure(t *testing.T) {
	s, root := newTestStore(t)
	first := touch(t, filepath.Join(root, "a.m4a"))
	second := filepath.Join(root, "b.m4a") // registered, never written
	a := s.Add("a", first, 1)
	s.Add("b", second, 1)

	dest := t.TempDir()
	if _, err := s.MigrateTo(dest); err == nil {
		t.Fatal("MigrateTo succeeded, want an error")
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("rollback did not restore %q: %v", first, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "a.m4a")); !os.IsNotExist(err) {
		t.Fatal("rollback left a copy in the destination")
	}
	if s.Root() != root {
		t.Fatalf("root = %q, want it unchanged at %q", s.Root(), root)
	}
	if got, _ := s.Get(a.ID); got.File != first {
		t.Fatalf("after rollback the recording points at %q, want %q", got.File, first)
	}
}

func TestSameDirFollowsSymlinksAndTrailingSlash(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if !SameDir(dir, link) {
		t.Fatalf("SameDir(%q, %q) = false, want true", dir, link)
	}
	if !SameDir(dir, dir+string(filepath.Separator)) {
		t.Fatalf("SameDir did not ignore a trailing separator")
	}
	if SameDir(dir, t.TempDir()) {
		t.Fatal("SameDir reported two different directories as equal")
	}
}
