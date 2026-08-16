package daemon

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/ryym/mop/internal/api"
	"github.com/ryym/mop/internal/version"
	"github.com/ryym/mop/web"
)

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/doc/open", s.handleOpen)
	mux.HandleFunc("POST /api/doc/update", s.handleUpdate)
	mux.HandleFunc("POST /api/doc/scroll", s.handleScroll)
	mux.HandleFunc("POST /api/doc/close", s.handleClose)
	mux.HandleFunc("GET /api/docs", s.handleDocs)
	mux.HandleFunc("POST /api/shutdown", s.handleShutdown)
	mux.HandleFunc("GET /api/status", s.handleStatus)

	mux.HandleFunc("GET /doc/{id}", s.handlePage)
	mux.HandleFunc("GET /doc/{id}/events", s.handleEvents)
	mux.HandleFunc("GET /doc/{id}/asset/{path...}", s.handleAsset)
	mux.HandleFunc("GET /static/", s.handleStatic)

	return s.checkRequest(mux)
}

// checkRequest is the DNS rebinding defence. A malicious page can point its
// own domain at 127.0.0.1 and reach this server from the browser, but the
// Host header stays the attacker's domain, and its Origin is not ours.
func (s *Server) checkRequest(next http.Handler) http.Handler {
	allowedHosts := map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", s.port): true,
		fmt.Sprintf("localhost:%d", s.port): true,
	}
	allowedOrigins := map[string]bool{
		fmt.Sprintf("http://127.0.0.1:%d", s.port): true,
		fmt.Sprintf("http://localhost:%d", s.port): true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHosts[r.Host] {
			writeError(w, http.StatusForbidden, "invalid Host header")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			// The CLI sends no Origin at all; only a browser does, and then
			// it must be ours.
			if origin := r.Header.Get("Origin"); origin != "" && !allowedOrigins[origin] {
				writeError(w, http.StatusForbidden, "invalid Origin header")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, api.ErrorResponse{Error: msg})
}

func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}

// validPath rejects anything that is not an absolute path. Resolving symlinks
// is the CLI's job; the daemon takes the path it is given as the identity.
func validPath(w http.ResponseWriter, path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		writeError(w, http.StatusBadRequest, "path must be an absolute path")
		return false
	}
	return true
}

func validRatio(w http.ResponseWriter, ratio *float64) bool {
	if ratio != nil && (*ratio < 0 || *ratio > 1) {
		writeError(w, http.StatusBadRequest, "viewportRatio must be between 0.0 and 1.0")
		return false
	}
	return true
}

func validLine(w http.ResponseWriter, line int) bool {
	if line < 1 {
		writeError(w, http.StatusBadRequest, "line must be 1 or greater")
		return false
	}
	return true
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	var req api.OpenRequest
	if !decode(w, r, &req) || !validPath(w, req.Path) {
		return
	}
	d, err := s.openDoc(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Info("document opened", "path", d.path)
	writeJSON(w, http.StatusOK, api.OpenResponse{URL: s.docURL(d.id)})
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var req api.UpdateRequest
	if !decode(w, r, &req) || !validPath(w, req.Path) || !validRatio(w, req.ViewportRatio) {
		return
	}
	if req.Line != nil && !validLine(w, *req.Line) {
		return
	}
	d, ok := s.getDoc(req.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "document is not open: "+req.Path)
		return
	}
	switch {
	case s.setContent(d, req.Content):
		s.hub.broadcast(d.id, newEvent("refresh", refreshPayload{
			Content:       req.Content,
			Line:          req.Line,
			ViewportRatio: req.ViewportRatio,
		}))
	case req.Line != nil:
		// Identical content: no refresh, but the position that came with it
		// is still honoured.
		s.hub.broadcast(d.id, newEvent("scroll", scrollPayload{
			Line:          *req.Line,
			ViewportRatio: req.ViewportRatio,
		}))
	}
	writeJSON(w, http.StatusOK, struct{}{})
}

func (s *Server) handleScroll(w http.ResponseWriter, r *http.Request) {
	var req api.ScrollRequest
	if !decode(w, r, &req) || !validPath(w, req.Path) ||
		!validLine(w, req.Line) || !validRatio(w, req.ViewportRatio) {
		return
	}
	d, ok := s.getDoc(req.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "document is not open: "+req.Path)
		return
	}
	s.hub.broadcast(d.id, newEvent("scroll", scrollPayload{
		Line:          req.Line,
		ViewportRatio: req.ViewportRatio,
	}))
	writeJSON(w, http.StatusOK, struct{}{})
}

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	var req api.CloseRequest
	if !decode(w, r, &req) || !validPath(w, req.Path) {
		return
	}
	if !s.closeDoc(req.Path) {
		writeError(w, http.StatusNotFound, "document is not open: "+req.Path)
		return
	}
	s.log.Info("document closed", "path", req.Path)
	writeJSON(w, http.StatusOK, struct{}{})
}

func (s *Server) handleDocs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.DocsResponse{Docs: s.listDocs()})
}

func (s *Server) handleShutdown(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, struct{}{})
	s.Stop()
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.StatusResponse{
		Version:       version.Version,
		Port:          s.port,
		UptimeSeconds: time.Since(s.started).Seconds(),
	})
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	d, ok := s.getDocByID(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	initial, err := initialJSON(s.snapshot(d))
	if err != nil {
		s.log.Warn("failed to encode initial content", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	err = s.page.Execute(w, map[string]any{
		"Title":   filepath.Base(d.path),
		"ID":      d.id,
		"Initial": initial,
	})
	if err != nil {
		s.log.Warn("failed to render page", "error", err)
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, ok := s.getDocByID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	sub := s.hub.subscribe(id)
	defer s.hub.unsubscribe(id, sub)

	// Send the current content immediately, so a reconnecting browser does
	// not need to track what it missed.
	writeEvent(w, flusher, newEvent("refresh", refreshPayload{Content: s.snapshot(d)}))

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.stop:
			return
		case ev, ok := <-sub.ch:
			if !ok {
				return
			}
			writeEvent(w, flusher, ev)
			if ev.name == "close" {
				return
			}
		case <-ping.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func writeEvent(w http.ResponseWriter, f http.Flusher, ev event) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.name, ev.data)
	f.Flush()
}

// handleAsset serves local files (images and such) relative to the
// document's base directory only. The daemon's own cwd is unrelated to the
// document, so relative paths must always resolve against that base.
func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	d, ok := s.getDocByID(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	rel := r.PathValue("path")
	target := filepath.Join(d.baseDir, filepath.FromSlash(rel))

	// Reject anything that escapes the base directory, symlinks included.
	base, err := filepath.EvalSymlinks(d.baseDir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if resolved != base && !strings.HasPrefix(resolved, base+string(filepath.Separator)) {
		writeError(w, http.StatusForbidden, "asset is outside the document directory")
		return
	}
	http.ServeFile(w, r, resolved)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/static/")
	data, err := fs.ReadFile(web.Dist, "dist/"+name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch filepath.Ext(name) {
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	// The asset URLs never change, so a cached bundle would survive a daemon
	// that was restarted precisely to pick up a new one. Revalidating on
	// localhost costs nothing.
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// initialJSON encodes the raw Markdown for the <script type="application/json">
// block in the page.
//
// encoding/json escapes <, > and & as \u003c and friends by default, so the
// document can never terminate the surrounding <script> element. The result is
// marked as template.JS because it is already JSON, and html/template would
// otherwise escape it a second time.
func initialJSON(content string) (template.JS, error) {
	data, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return "", err
	}
	return template.JS(data), nil
}
