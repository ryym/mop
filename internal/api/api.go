// Package api holds the request and response types of the control API.
//
// It exists so that the daemon (server) and the client agree on one
// definition. The package list in the architecture doc does not mention it;
// the alternative was duplicating the structs in both packages.
package api

// DefaultPort is the fixed port the daemon listens on unless told otherwise.
// It never falls back to another port, so URLs stay stable across restarts.
const DefaultPort = 7654

type OpenRequest struct {
	Path string `json:"path"`
}

type OpenResponse struct {
	URL string `json:"url"`
}

type UpdateRequest struct {
	Path          string   `json:"path"`
	Content       string   `json:"content"`
	Line          *int     `json:"line,omitempty"`
	ViewportRatio *float64 `json:"viewportRatio,omitempty"`
}

type ScrollRequest struct {
	Path          string   `json:"path"`
	Line          int      `json:"line"`
	ViewportRatio *float64 `json:"viewportRatio,omitempty"`
}

type CloseRequest struct {
	Path string `json:"path"`
}

type Doc struct {
	Path string `json:"path"`
	URL  string `json:"url"`
}

type DocsResponse struct {
	Docs []Doc `json:"docs"`
}

type StatusResponse struct {
	Version string `json:"version"`
	Port    int    `json:"port"`
	// UptimeSeconds is how long the daemon has been running.
	UptimeSeconds float64 `json:"uptimeSeconds"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
