package daemon

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryym/mop/internal/client"
)

// fileFixture is a repository with a document one level down, so that links
// can climb out of the document's directory without leaving the repository.
type fileFixture struct {
	c    *client.Client
	port int
	repo string // the repository root, holding .git
	doc  string // repo/docs/doc.md, open in the daemon
}

func newFileFixture(t *testing.T, withGit bool) *fileFixture {
	t.Helper()
	c, port := startDaemon(t)
	// Resolved so that paths compare equal to what the daemon resolves; on
	// macOS the temp dir sits behind the /var -> /private/var symlink.
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if withGit {
		mkdir(t, filepath.Join(repo, ".git"))
	}
	doc := filepath.Join(repo, "docs", "doc.md")
	writeFile(t, doc, "# Doc\n")
	if _, err := c.Open(doc); err != nil {
		t.Fatal(err)
	}
	return &fileFixture{c: c, port: port, repo: repo, doc: doc}
}

// get requests a link relative to the document. Redirects are not followed,
// and site is sent as Sec-Fetch-Site unless empty.
func (f *fileFixture) get(t *testing.T, rel, site string) *http.Response {
	t.Helper()
	u := fmt.Sprintf("http://127.0.0.1:%d/doc/%s/file?path=%s", f.port, docID(f.doc), url.QueryEscape(rel))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	if site != "" {
		req.Header.Set("Sec-Fetch-Site", site)
	}
	noRedirect := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	res, err := noRedirect.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wantStatus(t *testing.T, res *http.Response, want int) {
	t.Helper()
	if res.StatusCode != want {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, want, body)
	}
}

func TestFileRedirectsToLinkedDocument(t *testing.T) {
	f := newFileFixture(t, true)
	other := filepath.Join(f.repo, "README.md")
	writeFile(t, other, "# Readme\n")

	res := f.get(t, "../README.md", "same-origin")
	wantStatus(t, res, http.StatusSeeOther)
	if got, want := res.Header.Get("Location"), "/doc/"+docID(other); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	docs, err := f.c.Docs()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, d := range docs.Docs {
		found = found || d.Path == other
	}
	if !found {
		t.Errorf("linked document is not registered: %+v", docs.Docs)
	}
}

// A document reached through a symlink gets the identity of its target, as it
// would from the CLI.
func TestFileResolvesSymlinkedDocument(t *testing.T) {
	f := newFileFixture(t, true)
	other := filepath.Join(f.repo, "real.md")
	writeFile(t, other, "# Real\n")
	if err := os.Symlink(other, filepath.Join(f.repo, "docs", "link.md")); err != nil {
		t.Fatal(err)
	}

	res := f.get(t, "link.md", "")
	wantStatus(t, res, http.StatusSeeOther)
	if got, want := res.Header.Get("Location"), "/doc/"+docID(other); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

func TestFileServesOtherFilesSandboxed(t *testing.T) {
	f := newFileFixture(t, true)
	writeFile(t, filepath.Join(f.repo, "src", "main.ts"), "const a = 1;\n")

	res := f.get(t, "../src/main.ts", "same-origin")
	wantStatus(t, res, http.StatusOK)
	body, _ := io.ReadAll(res.Body)
	if string(body) != "const a = 1;\n" {
		t.Errorf("body = %q", body)
	}
	if got := res.Header.Get("Content-Security-Policy"); got != "sandbox" {
		t.Errorf("Content-Security-Policy = %q, want sandbox", got)
	}
	if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestFileContentType(t *testing.T) {
	f := newFileFixture(t, true)
	// 511 ASCII bytes and then a 3 byte character, cut by the 512 byte sniff.
	cut := fmt.Sprintf("%0511d", 0) + "あ"

	cases := []struct {
		name, content, wantType, wantDisposition string
	}{
		{"a.png", "\x89PNG", "image/png", ""},
		{"a.JPG", "x", "image/jpeg", ""},
		{"a.svg", "<svg/>", "image/svg+xml", ""},
		{"a.bmp", "BM\x00\x00", "image/bmp", ""},
		{"a.pdf", "%PDF-", "application/pdf", "attachment"},
		{"a.ts", "const a = 1;", "text/plain; charset=utf-8", ""},
		{"a.sh", "#!/bin/sh\n", "text/plain; charset=utf-8", ""},
		{"a.html", "<script>x</script>", "text/plain; charset=utf-8", ""},
		{"a.mdx", "# x", "text/plain; charset=utf-8", ""},
		{"ja.txt", "日本語", "text/plain; charset=utf-8", ""},
		{"empty", "", "text/plain; charset=utf-8", ""},
		{"cut.txt", cut, "text/plain; charset=utf-8", ""},
		{"nul.bin", "a\x00b", "application/octet-stream", ""},
		{"latin1.txt", "caf\xe9", "application/octet-stream", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeFile(t, filepath.Join(f.repo, "docs", tc.name), tc.content)
			res := f.get(t, tc.name, "")
			wantStatus(t, res, http.StatusOK)
			if got := res.Header.Get("Content-Type"); got != tc.wantType {
				t.Errorf("Content-Type = %q, want %q", got, tc.wantType)
			}
			if got := res.Header.Get("Content-Disposition"); got != tc.wantDisposition {
				t.Errorf("Content-Disposition = %q, want %q", got, tc.wantDisposition)
			}
		})
	}
}

func TestFileStaysInsideServingRoot(t *testing.T) {
	t.Run("repository", func(t *testing.T) {
		f := newFileFixture(t, true)
		// Next to the repository, inside the test's own temp area.
		outside := filepath.Join(filepath.Dir(f.repo), "outside.txt")
		writeFile(t, outside, "secret")

		wantStatus(t, f.get(t, "../../outside.txt", ""), http.StatusForbidden)
	})
	t.Run("document directory without a repository", func(t *testing.T) {
		f := newFileFixture(t, false)
		writeFile(t, filepath.Join(f.repo, "sibling.txt"), "x")

		wantStatus(t, f.get(t, "../sibling.txt", ""), http.StatusForbidden)
	})
	t.Run("symlink pointing outside", func(t *testing.T) {
		f := newFileFixture(t, true)
		outside := filepath.Join(t.TempDir(), "secret.txt")
		writeFile(t, outside, "secret")
		if err := os.Symlink(outside, filepath.Join(f.repo, "docs", "link.txt")); err != nil {
			t.Fatal(err)
		}

		wantStatus(t, f.get(t, "link.txt", ""), http.StatusForbidden)
	})
}

// .git is a file in worktrees and submodules.
func TestFileRootFromGitFile(t *testing.T) {
	f := newFileFixture(t, false)
	// Created after the document is opened, which the serving root follows.
	writeFile(t, filepath.Join(f.repo, ".git"), "gitdir: /elsewhere\n")
	writeFile(t, filepath.Join(f.repo, "top.txt"), "x")

	wantStatus(t, f.get(t, "../top.txt", ""), http.StatusOK)
}

func TestFileRejectsBadPaths(t *testing.T) {
	f := newFileFixture(t, true)
	mkdir(t, filepath.Join(f.repo, "docs", "sub"))

	for rel, want := range map[string]int{
		"missing.png": http.StatusNotFound,
		"sub":         http.StatusNotFound,
		"":            http.StatusBadRequest,
		"/etc/hosts":  http.StatusBadRequest,
		"../../../..": http.StatusForbidden,
	} {
		t.Run("path="+rel, func(t *testing.T) {
			wantStatus(t, f.get(t, rel, ""), want)
		})
	}
}
