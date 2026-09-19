package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
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
