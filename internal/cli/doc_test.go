package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryym/mop/internal/client"
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

func TestNoDaemonExitsWithNoPreview(t *testing.T) {
	// An empty state dir means no daemon, and these commands must not start one.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	path := filepath.Join(t.TempDir(), "notes.md")
	for _, args := range [][]string{
		{"scroll", path, "--line", "1"},
		{"close", path},
	} {
		if got := Run(args); got != exitNoPreview {
			t.Errorf("Run(%q) = %d, want %d", args, got, exitNoPreview)
		}
	}
}

func TestAsNoPreview(t *testing.T) {
	notFound := fmt.Errorf("%w: %s", client.ErrNotFound, "notes.md")
	other := errors.New("boom")
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{errNoDaemon, true},
		{notFound, true},
		{other, false},
	} {
		var np *noPreviewError
		if got := errors.As(asNoPreview(tc.err), &np); got != tc.want {
			t.Errorf("asNoPreview(%v) marked = %v, want %v", tc.err, got, tc.want)
		}
	}
	if asNoPreview(nil) != nil {
		t.Error("asNoPreview(nil) != nil")
	}
}
