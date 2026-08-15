// Package client is the HTTP client for the daemon's control API.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ryym/mop/internal/api"
)

// ErrNotFound is returned when the daemon does not know the given document.
var ErrNotFound = errors.New("document is not open")

type Client struct {
	port int
	http *http.Client
}

func New(port int) *Client {
	return &Client{
		port: port,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Port() int { return c.port }

func (c *Client) url(path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", c.port, path)
}

func (c *Client) Status() (api.StatusResponse, error) {
	var res api.StatusResponse
	err := c.do(http.MethodGet, "/api/status", nil, &res)
	return res, err
}

func (c *Client) Open(path string) (api.OpenResponse, error) {
	var res api.OpenResponse
	err := c.do(http.MethodPost, "/api/doc/open", api.OpenRequest{Path: path}, &res)
	return res, err
}

func (c *Client) Update(req api.UpdateRequest) error {
	return c.do(http.MethodPost, "/api/doc/update", req, nil)
}

func (c *Client) Scroll(req api.ScrollRequest) error {
	return c.do(http.MethodPost, "/api/doc/scroll", req, nil)
}

func (c *Client) CloseDoc(path string) error {
	return c.do(http.MethodPost, "/api/doc/close", api.CloseRequest{Path: path}, nil)
}

func (c *Client) Docs() (api.DocsResponse, error) {
	var res api.DocsResponse
	err := c.do(http.MethodGet, "/api/docs", nil, &res)
	return res, err
}

func (c *Client) Shutdown() error {
	return c.do(http.MethodPost, "/api/shutdown", struct{}{}, nil)
}

func (c *Client) do(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, c.url(path), reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		var errRes api.ErrorResponse
		_ = json.NewDecoder(res.Body).Decode(&errRes)
		msg := errRes.Error
		if msg == "" {
			msg = res.Status
		}
		if res.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%w: %s", ErrNotFound, msg)
		}
		return errors.New(msg)
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	_, _ = io.Copy(io.Discard, res.Body)
	return nil
}
