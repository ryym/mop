// Package docpath holds the decisions about document paths that the CLI and the
// daemon must agree on.
package docpath

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Resolve makes a path absolute and resolves symlinks, which is what gives a
// document its identity.
func Resolve(path string) (string, error) {
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

// IsDocument reports whether a file is one mop previews, judged by its
// extension alone. .mdx is left out because its JSX cannot be rendered here,
// and rarer spellings such as .mdown are left out to keep the rule short.
func IsDocument(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return true
	}
	return false
}
