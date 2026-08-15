package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// docID is the identifier used in preview URLs: the first 12 hex characters
// of the SHA-256 of the absolute path.
//
// Deriving it from the path means the same file always gets the same URL, so
// browser tabs survive a daemon restart, and no counter has to be persisted.
// The absolute path also stays out of the URL.
func docID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:])[:12]
}

type document struct {
	id      string
	path    string
	baseDir string // the file's parent directory; used for relative assets
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
