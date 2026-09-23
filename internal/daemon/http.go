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

// handler wires up the two surfaces the daemon serves: the control plane under
// /api/, spoken by the CLI, and the preview surface, spoken by the browser.
// Documents are named by absolute path on the former and by id on the latter.
func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()

	// Control plane. Requests and responses are the types in internal/api.
	mux.HandleFunc("POST /api/doc/open", s.handleOpen)     // register a document, return its preview URL
	mux.HandleFunc("POST /api/doc/update", s.handleUpdate) // replace the content, optionally focusing a line
	mux.HandleFunc("POST /api/doc/scroll", s.handleScroll) // move the preview to a source line
	mux.HandleFunc("POST /api/doc/close", s.handleClose)   // unregister and tell the browser it is closed
	mux.HandleFunc("GET /api/docs", s.handleDocs)          // list open documents
	mux.HandleFunc("POST /api/shutdown", s.handleShutdown) // stop the daemon
	mux.HandleFunc("GET /api/status", s.handleStatus)      // version, port, uptime; also the liveness probe

	// Preview surface.
	mux.HandleFunc("GET /doc/{id}", s.handlePage)                  // the preview page
	mux.HandleFunc("GET /doc/{id}/events", s.handleEvents)         // its SSE stream
	mux.HandleFunc("GET /doc/{id}/asset/{path...}", s.handleAsset) // local files next to the document
	mux.HandleFunc("GET /static/", s.handleStatic)                 // the embedded bundle

	return s.checkRequest(mux)
}

// checkRequest rejects requests addressed to a Host other than the daemon's
// own, and any browser request to the control plane, including one from the
// daemon's own origin.
func (s *Server) checkRequest(next http.Handler) http.Handler {
	allowedHosts := map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", s.port): true,
		fmt.Sprintf("localhost:%d", s.port): true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject a foreign Host to defend against DNS rebinding: a malicious
		// page can point its own domain at 127.0.0.1 and reach this server,
		// but the Host header stays the attacker's domain.
		if !allowedHosts[r.Host] {
			writeError(w, http.StatusForbidden, "invalid Host header")
			return
		}
		// Close the control plane to browsers, our own origin included,
		// because a script running there (e.g. a local HTML file opened as an
		// asset) could otherwise open and read any file the user can. The CLI
		// sends neither header. Check Sec-Fetch-Site too because browsers may
		// omit Origin on same-origin GETs, while current ones send
		// Sec-Fetch-Site on every request.
		if strings.HasPrefix(r.URL.Path, "/api/") &&
			(r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "") {
			writeError(w, http.StatusForbidden, "the control plane does not accept browser requests")
			return
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
		"Title":   d.title(),
		"Path":    d.displayPath(),
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
	// Revalidate rather than serve from cache: an image edited next to the
	// document has to show up in the preview. ServeFile still answers 304
	// while the file is unchanged.
	w.Header().Set("Cache-Control", "no-cache")
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
func initialJSON(content string) (template.JS, error) {
	// encoding/json escapes <, > and & as \u003c and friends by default, so the
	// document can never terminate the surrounding <script> element.
	data, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return "", err
	}
	// Mark it as template.JS because it is already JSON; html/template would
	// otherwise escape it a second time.
	return template.JS(data), nil
}
