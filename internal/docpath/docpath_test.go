package docpath

import "testing"

func TestIsDocument(t *testing.T) {
	for path, want := range map[string]bool{
		"/a/README.md":     true,
		"/a/README.MD":     true,
		"/a/x.markdown":    true,
		"/a/x.txt":         false,
		"/a/x.mdx":         false,
		"/a/Makefile":      false,
		"/a/x.md/Makefile": false,
	} {
		if got := IsDocument(path); got != want {
			t.Errorf("IsDocument(%q) = %v, want %v", path, got, want)
		}
	}
}
