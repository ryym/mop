package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// docID is the identifier used in preview URLs. Deriving it from the absolute
// path means the same file always gets the same URL, without persisting a
// counter and without putting the path itself in the URL.
func docID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:])[:12]
}

type document struct {
	id      string
	path    string
	baseDir string // resolves the document's relative assets
	content string // the raw Markdown, as received or read from disk
}

func newDocument(path string) *document {
	return &document{
		id:      docID(path),
		path:    path,
		baseDir: filepath.Dir(path),
	}
}

func (d *document) readFromDisk() (string, error) {
	data, err := os.ReadFile(d.path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", d.path, err)
	}
	return string(data), nil
}

// displayPath is the document's path as shown to the reader: the absolute
// path, with the home directory abbreviated to ~ so that long paths stay
// readable.
func (d *document) displayPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return d.path
	}
	if d.path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(d.path, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return d.path
}

// title names the document in the browser tab, where the full path does not
// fit. The parent directory is kept because file names alone (README.md,
// index.md) rarely tell two tabs apart.
func (d *document) title() string {
	return filepath.Join(filepath.Base(d.baseDir), filepath.Base(d.path))
}
