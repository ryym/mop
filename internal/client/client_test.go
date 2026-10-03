package client

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ryym/mop/internal/api"
)

func TestNotFoundKeepsDaemonMessage(t *testing.T) {
	const msg = "document is not open: /tmp/notes.md"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(api.ErrorResponse{Error: msg})
	}))
	defer srv.Close()

	c := New(srv.Listener.Addr().(*net.TCPAddr).Port)
	err := c.Scroll(api.ScrollRequest{Path: "/tmp/notes.md", Line: 1})
	if _, ok := errors.AsType[*NotFoundError](err); !ok {
		t.Fatalf("err = %v, want a NotFoundError", err)
	}
	if err.Error() != msg {
		t.Errorf("message = %q, want %q", err.Error(), msg)
	}
}
