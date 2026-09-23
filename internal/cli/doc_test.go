package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenRejectsNonMarkdown(t *testing.T) {
	// Point the state file at a temp dir so that, if the Markdown check in
	// runOpen ever ends up after state.Connect, the test fails without finding
	// and replacing the user's running daemon with the test binary.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runOpen([]string{path})
	if err == nil || !strings.Contains(err.Error(), "not a Markdown file") {
		t.Fatalf("err = %v, want a not-a-Markdown-file error", err)
	}
}
