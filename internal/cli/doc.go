package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ryym/mop/internal/api"
	"github.com/ryym/mop/internal/client"
	"github.com/ryym/mop/internal/docpath"
	"github.com/ryym/mop/internal/state"
)

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

var errNoDaemon = errors.New("no preview is running")

// connectExisting talks to a daemon that is already running, returning
// errNoDaemon instead of starting one. Commands that act on a registered
// document cannot be made to succeed by a fresh daemon, so starting one would
// only add a multi second stall to what an editor calls on every keystroke.
func connectExisting() (*client.Client, error) {
	if _, err := state.Load(); errors.Is(err, state.ErrNoState) {
		return nil, errNoDaemon
	}
	return state.Connect()
}

// positionFlags are the --line / --viewport-ratio pair shared by update and
// scroll. linePtr and ratioPtr report an omitted flag as nil, which is how the
// control API distinguishes it from a given value.
type positionFlags struct {
	line  int
	ratio float64
	fs    *flag.FlagSet
}

func addPositionFlags(fs *flag.FlagSet) *positionFlags {
	p := &positionFlags{fs: fs}
	fs.IntVar(&p.line, "line", 0, "source line to focus (1 based)")
	fs.Float64Var(&p.ratio, "viewport-ratio", -1, "where to place the line (0.0-1.0)")
	return p
}

func (p *positionFlags) linePtr() *int {
	if p.line == 0 {
		return nil
	}
	return &p.line
}

func (p *positionFlags) ratioPtr() *float64 {
	if p.ratio < 0 {
		return nil
	}
	return &p.ratio
}

func runOpen(args []string) error {
	fs := newFlagSet("open")
	path, err := parseFileArgs(fs, args)
	if err != nil {
		return err
	}
	// Checked before connecting so that a mistyped file never costs a daemon
	// start. The daemon checks again for callers other than this CLI.
	if !docpath.IsDocument(path) {
		return fmt.Errorf("cannot open %s: not a Markdown file", path)
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("cannot open %s: %w", path, err)
	}

	c, err := state.Connect()
	if err != nil {
		return err
	}
	res, err := c.Open(path)
	if err != nil {
		return err
	}
	fmt.Println(res.URL)
	if err := openBrowser(res.URL); err != nil {
		fmt.Fprintf(os.Stderr, "mop: failed to open a browser: %v\n", err)
	}
	return nil
}

func runUpdate(args []string) error {
	fs := newFlagSet("update")
	pos := addPositionFlags(fs)
	path, err := parseFileArgs(fs, args)
	if err != nil {
		return err
	}
	// The file itself is never read: <file> only identifies the document.
	content, err := readStdin()
	if err != nil {
		return err
	}
	c, err := connectExisting()
	if err != nil {
		return err
	}
	return c.Update(api.UpdateRequest{
		Path:          path,
		Content:       content,
		Line:          pos.linePtr(),
		ViewportRatio: pos.ratioPtr(),
	})
}

func runScroll(args []string) error {
	fs := newFlagSet("scroll")
	pos := addPositionFlags(fs)
	path, err := parseFileArgs(fs, args)
	if err != nil {
		return err
	}
	if pos.line < 1 {
		return errors.New("--line is required and must be 1 or greater")
	}
	c, err := connectExisting()
	if err != nil {
		return err
	}
	return c.Scroll(api.ScrollRequest{
		Path:          path,
		Line:          pos.line,
		ViewportRatio: pos.ratioPtr(),
	})
}

func runClose(args []string) error {
	fs := newFlagSet("close")
	path, err := parseFileArgs(fs, args)
	if err != nil {
		return err
	}
	c, err := connectExisting()
	if errors.Is(err, errNoDaemon) {
		return nil // Nothing is open, so there is nothing to close.
	}
	if err != nil {
		return err
	}
	return c.CloseDoc(path)
}

func runList(args []string) error {
	fs := newFlagSet("list")
	asJSON := fs.Bool("json", false, "print machine readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Listing must not start a daemon: with none running, nothing is open.
	c, err := connectExisting()
	if errors.Is(err, errNoDaemon) {
		if *asJSON {
			fmt.Println(`{"docs":[]}`)
		}
		return nil
	}
	if err != nil {
		return err
	}
	res, err := c.Docs()
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		return enc.Encode(res)
	}
	for _, d := range res.Docs {
		fmt.Printf("%s\t%s\n", d.Path, d.URL)
	}
	return nil
}
