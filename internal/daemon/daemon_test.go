package daemon

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ryym/mop/internal/api"
	"github.com/ryym/mop/internal/client"
)

// startDaemon runs a real daemon on a free port, with the state file
// redirected into the test's temp dir.
func startDaemon(t *testing.T) (*client.Client, int) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	s, err := New(Options{
		Port:   port,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = s.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	c := client.New(port)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := c.Status(); err == nil {
			return c, port
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("daemon did not start")
	return nil, 0
}

func writeDoc(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenListClose(t *testing.T) {
	c, port := startDaemon(t)
	path := writeDoc(t, "# Hello\n")

	res, err := c.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("http://127.0.0.1:%d/doc/%s", port, docID(path))
	if res.URL != want {
		t.Fatalf("url = %q, want %q", res.URL, want)
	}

	// Opening again returns the same URL rather than a second document.
	again, err := c.Open(path)
	if err != nil || again.URL != want {
		t.Fatalf("reopen: %v %q", err, again.URL)
	}

	docs, err := c.Docs()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs.Docs) != 1 || docs.Docs[0].Path != path {
		t.Fatalf("unexpected docs: %+v", docs.Docs)
	}

	if err := c.CloseDoc(path); err != nil {
		t.Fatal(err)
	}
	docs, _ = c.Docs()
	if len(docs.Docs) != 0 {
		t.Fatalf("expected no docs, got %+v", docs.Docs)
	}
}

func TestUpdateAndScrollRequireOpenDocument(t *testing.T) {
	c, _ := startDaemon(t)
	path := writeDoc(t, "# Hello\n")

	if err := c.Update(api.UpdateRequest{Path: path, Content: "x"}); err == nil {
		t.Fatal("expected an error for an unopened document")
	}
	if err := c.Scroll(api.ScrollRequest{Path: path, Line: 1}); err == nil {
		t.Fatal("expected an error for an unopened document")
	}
}

// The page embeds the raw Markdown, not HTML: rendering is the browser's job.
// The embedded JSON must also be unable to break out of the <script> element.
func TestPageEmbedsRawMarkdown(t *testing.T) {
	c, port := startDaemon(t)
	path := writeDoc(t, "# Hello\n</script><b>x</b>\n")
	if _, err := c.Open(path); err != nil {
		t.Fatal(err)
	}

	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/doc/%s", port, docID(path)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), `{"content":"# Hello\n`) {
		t.Fatalf("initial content missing:\n%s", body)
	}
	if strings.Contains(string(body), "</script><b>") {
		t.Fatalf("embedded content was not escaped:\n%s", body)
	}
}

func TestRejectsForeignHost(t *testing.T) {
	c, port := startDaemon(t)
	_ = c

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/api/docs", port), nil)
	req.Host = "evil.example.com"
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", res.StatusCode)
	}
}

func TestRejectsForeignOrigin(t *testing.T) {
	c, port := startDaemon(t)
	_ = c

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/api/docs", port), nil)
	req.Header.Set("Origin", "https://evil.example.com")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", res.StatusCode)
	}
}

func TestSSEDeliversRefreshAndScroll(t *testing.T) {
	c, port := startDaemon(t)
	path := writeDoc(t, "# Hello\n")
	if _, err := c.Open(path); err != nil {
		t.Fatal(err)
	}

	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/doc/%s/events", port, docID(path)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	reader := bufio.NewReader(res.Body)

	// The initial refresh carries the current content.
	if ev, data := readEvent(t, reader); ev != "refresh" || !strings.Contains(data, "Hello") {
		t.Fatalf("first event = %q %q", ev, data)
	}

	if err := c.Update(api.UpdateRequest{Path: path, Content: "# Updated\n"}); err != nil {
		t.Fatal(err)
	}
	if ev, data := readEvent(t, reader); ev != "refresh" || !strings.Contains(data, "Updated") {
		t.Fatalf("update event = %q %q", ev, data)
	}

	if err := c.Scroll(api.ScrollRequest{Path: path, Line: 1}); err != nil {
		t.Fatal(err)
	}
	if ev, data := readEvent(t, reader); ev != "scroll" || !strings.Contains(data, `"line":1`) {
		t.Fatalf("scroll event = %q %q", ev, data)
	}

	if err := c.CloseDoc(path); err != nil {
		t.Fatal(err)
	}
	if ev, _ := readEvent(t, reader); ev != "close" {
		t.Fatalf("close event = %q", ev)
	}
}

// readEvent reads one "event:/data:" pair, skipping keep-alive comments.
func readEvent(t *testing.T, r *bufio.Reader) (name, data string) {
	t.Helper()
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("reading SSE: %v", err)
		}
		line = strings.TrimRight(line, "\n")
		switch {
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			return name, strings.TrimPrefix(line, "data: ")
		}
	}
}
