package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitChange(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case p := <-ch:
		return p
	case <-time.After(3 * time.Second):
		t.Fatal("no change reported")
		return ""
	}
}

func TestWatchInPlaceAndRenameSave(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(file, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ch := make(chan string, 8)
	w, err := New(func(p string) { ch <- p })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(file); err != nil {
		t.Fatal(err)
	}

	// In-place write.
	if err := os.WriteFile(file, []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := waitChange(t, ch); got != file {
		t.Fatalf("got %q, want %q", got, file)
	}

	// Save via temp file + rename, the way many editors do it.
	tmp := filepath.Join(dir, ".doc.md.swp")
	if err := os.WriteFile(tmp, []byte("three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, file); err != nil {
		t.Fatal(err)
	}
	if got := waitChange(t, ch); got != file {
		t.Fatalf("got %q, want %q", got, file)
	}
}

func TestRemoveStopsReporting(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(file, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ch := make(chan string, 8)
	w, err := New(func(p string) { ch <- p })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(file); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-ch:
		t.Fatalf("unexpected change for %q", p)
	case <-time.After(300 * time.Millisecond):
	}
}
