// Package daemon is the HTTP server: control API, preview pages, SSE hub,
// document registry and file watching.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ryym/mop/internal/api"
	"github.com/ryym/mop/internal/state"
	"github.com/ryym/mop/internal/version"
	"github.com/ryym/mop/internal/watch"
	"github.com/ryym/mop/web"
)

const (
	// idleTimeout is how long the daemon stays alive with no browser
	// connected: closing the last tab is what ends a session.
	idleTimeout = 10 * time.Minute
	idleCheck   = 30 * time.Second
)

type Options struct {
	Port   int
	Logger *slog.Logger
}

type Server struct {
	port    int
	log     *slog.Logger
	hub     *hub
	watcher *watch.Watcher
	page    *template.Template
	started time.Time

	// mu guards docs. The workload is one user previewing a handful of
	// files, so a single lock is enough.
	mu   sync.Mutex
	docs map[string]*document

	stopOnce sync.Once
	stop     chan struct{}
}

func New(opts Options) (*Server, error) {
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	if err := web.CheckBundle(); err != nil {
		return nil, err
	}
	page, err := template.New("page").Parse(web.PageTemplate)
	if err != nil {
		return nil, err
	}
	s := &Server{
		port:    opts.Port,
		log:     log,
		hub:     newHub(),
		page:    page,
		started: time.Now(),
		docs:    map[string]*document{},
		stop:    make(chan struct{}),
	}
	w, err := watch.New(s.onFileChanged)
	if err != nil {
		return nil, err
	}
	s.watcher = w
	return s, nil
}

// Run listens and serves until the daemon is asked to stop, goes idle, or the
// context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// No fallback port: a stable port is what keeps preview URLs valid.
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	defer s.watcher.Close()

	if err := state.Save(state.State{
		Port:    s.port,
		PID:     os.Getpid(),
		Version: version.Version,
		Build:   state.BuildID(),
	}); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}
	defer s.clearState()

	srv := &http.Server{Handler: s.handler()}
	errc := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
			return
		}
		errc <- nil
	}()

	s.log.Info("daemon started", "port", s.port, "version", version.Version)

	ticker := time.NewTicker(idleCheck)
	defer ticker.Stop()

	var runErr error
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-s.stop:
			break loop
		case err := <-errc:
			runErr = err
			break loop
		case <-ticker.C:
			if s.hub.idleSince() > idleTimeout {
				s.log.Info("no browser connected; shutting down", "idle", idleTimeout)
				break loop
			}
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	s.log.Info("daemon stopped")
	return runErr
}

// Stop asks the daemon to shut down.
func (s *Server) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

// clearState removes the state file only if it still describes this process,
// so a daemon started later is not orphaned by a slow exit here.
func (s *Server) clearState() {
	cur, err := state.Load()
	if err == nil && cur.PID != os.Getpid() {
		return
	}
	_ = state.Clear()
}

func (s *Server) docURL(id string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/doc/%s", s.port, id)
}

// openDoc registers a document, reading it from disk, and starts watching it.
// Opening an already open document just returns it.
func (s *Server) openDoc(path string) (*document, error) {
	s.mu.Lock()
	if d, ok := s.docs[docID(path)]; ok {
		s.mu.Unlock()
		return d, nil
	}
	s.mu.Unlock()

	d := newDocument(path)
	content, err := d.readFromDisk()
	if err != nil {
		return nil, err
	}
	d.content = content

	s.mu.Lock()
	// Another request may have registered it in the meantime.
	if existing, ok := s.docs[d.id]; ok {
		s.mu.Unlock()
		return existing, nil
	}
	s.docs[d.id] = d
	s.mu.Unlock()

	if err := s.watcher.Add(path); err != nil {
		s.log.Warn("failed to watch file", "path", path, "error", err)
	}
	return d, nil
}

func (s *Server) getDoc(path string) (*document, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.docs[docID(path)]
	return d, ok
}

func (s *Server) getDocByID(id string) (*document, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.docs[id]
	return d, ok
}

func (s *Server) listDocs() []api.Doc {
	s.mu.Lock()
	defer s.mu.Unlock()
	docs := make([]api.Doc, 0, len(s.docs))
	for _, d := range s.docs {
		docs = append(docs, api.Doc{Path: d.path, URL: s.docURL(d.id)})
	}
	return docs
}

func (s *Server) closeDoc(path string) bool {
	id := docID(path)
	s.mu.Lock()
	d, ok := s.docs[id]
	if ok {
		delete(s.docs, id)
	}
	s.mu.Unlock()
	if !ok {
		return false
	}
	_ = s.watcher.Remove(d.path)
	s.hub.broadcast(id, newEvent("close", struct{}{}))
	return true
}

// setContent stores the new text and reports whether it differs from what the
// document already had, so that identical text can be dropped without a
// refresh.
func (s *Server) setContent(d *document, content string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.content == content {
		return false
	}
	d.content = content
	return true
}

func (s *Server) snapshot(d *document) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return d.content
}

// onFileChanged is the watcher callback: it re-reads the file and pushes it to
// the browsers watching it.
func (s *Server) onFileChanged(path string) {
	d, ok := s.getDoc(path)
	if !ok {
		return
	}
	content, err := d.readFromDisk()
	if err != nil {
		s.log.Warn("failed to read changed file", "path", path, "error", err)
		return
	}
	if !s.setContent(d, content) {
		return
	}
	// No line: a file-watch refresh must not move the reader's viewport.
	s.hub.broadcast(d.id, newEvent("refresh", refreshPayload{Content: content}))
}

// refreshPayload carries raw Markdown, not HTML: rendering is the browser's job.
type refreshPayload struct {
	Content       string   `json:"content"`
	Line          *int     `json:"line,omitempty"`
	ViewportRatio *float64 `json:"viewportRatio,omitempty"`
}

type scrollPayload struct {
	Line          int      `json:"line"`
	ViewportRatio *float64 `json:"viewportRatio,omitempty"`
}
