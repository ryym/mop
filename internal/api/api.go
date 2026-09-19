// Package api holds the request and response types of the control API, so that
// the daemon and the client share one definition of them.
package api

// DefaultPort is the port the daemon listens on unless told otherwise.
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
	Version       string  `json:"version"`
	Port          int     `json:"port"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
