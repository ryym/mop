// Package cli implements the subcommands.
//
// It holds no state of its own: it resolves paths, talks to the daemon, and
// formats what comes back.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ryym/mop/internal/version"
)

const usage = `mop - live Markdown preview in the browser

Usage:
  mop open <file>                       Open a preview
  mop update <file> [--line N]          Replace the previewed content with stdin
  mop scroll <file> --line N            Move the preview to a source line
  mop close <file>                      Close a preview
  mop list [--json]                     List open documents
  mop daemon start [--port N] [--foreground]
  mop daemon stop

Options:
  --line N              Source line to focus (1 based)
  --viewport-ratio R    Where to place that line: 0.0 top, 1.0 bottom (default 0.5)
  --help                Show this help
  --version             Show the version
`

// Run dispatches a subcommand and returns the process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}

	switch args[0] {
	case "--help", "-h", "help":
		fmt.Print(usage)
		return 0
	case "--version":
		fmt.Println(version.Version)
		return 0
	}

	var err error
	switch args[0] {
	case "open":
		err = runOpen(args[1:])
	case "update":
		err = runUpdate(args[1:])
	case "scroll":
		err = runScroll(args[1:])
	case "close":
		err = runClose(args[1:])
	case "list":
		err = runList(args[1:])
	case "daemon":
		err = runDaemon(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", args[0], usage)
		return 2
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "mop: "+err.Error())
		return 1
	}
	return 0
}

// resolvePath makes a path absolute and resolves symlinks, which is what
// gives a document its identity. The daemon trusts whatever it receives.
func resolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// The file may be gone (e.g. `mop close` after a delete). The
		// absolute path is still a usable identifier for the daemon.
		if errors.Is(err, os.ErrNotExist) {
			return abs, nil
		}
		return "", err
	}
	return resolved, nil
}

// parseFileArgs parses flags that may appear after the file argument, which
// is how the CLI spec writes them (`mop scroll <file> --line N`). Go's flag
// package stops at the first positional argument, so parsing is resumed after
// pulling it out.
func parseFileArgs(fs *flag.FlagSet, args []string) (string, error) {
	var file string
	for {
		if err := fs.Parse(args); err != nil {
			return "", err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		if file != "" {
			return "", fmt.Errorf("unexpected argument: %s", rest[0])
		}
		file = rest[0]
		args = rest[1:]
	}
	if file == "" {
		return "", errors.New("a file argument is required")
	}
	return resolvePath(file)
}

func readStdin() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("failed to read stdin: %w", err)
	}
	return string(data), nil
}

// openBrowser opens a URL with the platform's default handler. Failing to do
// so is not fatal: the URL is printed anyway.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
