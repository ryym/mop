package daemon

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// imageTypes are the files a browser displays as they are. Only images are
// listed: anything else is either text, shown as such, or downloaded. Types
// are decided here rather than by http.ServeFile, which also consults the OS
// MIME table: that differs between machines and, on macOS, makes .ts and .sh
// files download instead of display.
var imageTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
}

// sniffLen is how much of a file looksLikeText examines, the same amount
// http.DetectContentType looks at.
const sniffLen = 512

// setFileHeaders decides how a linked file is presented, its Content-Type and
// for PDFs a download, from its name and content.
func setFileHeaders(h http.Header, path string, head []byte, size int64) {
	ext := strings.ToLower(filepath.Ext(path))
	if t, ok := imageTypes[ext]; ok {
		h.Set("Content-Type", t)
		return
	}
	// Make PDFs download instead of opening in the browser. Files are served
	// sandboxed (see handleFile), and browsers refuse to run their PDF viewer
	// in a sandboxed document.
	if ext == ".pdf" {
		h.Set("Content-Type", "application/pdf")
		h.Set("Content-Disposition", "attachment")
		return
	}

	// HTML falls through to here on purpose: shown as source, it is readable,
	// whereas rendered it would mostly lose its relative styles and images.
	if looksLikeText(head, size > int64(len(head))) {
		h.Set("Content-Type", "text/plain; charset=utf-8")
	} else {
		h.Set("Content-Type", "application/octet-stream")
	}
}

// readHead returns the first sniffLen bytes of a file, or all of a shorter one.
func readHead(r io.Reader) ([]byte, error) {
	buf := make([]byte, sniffLen)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return buf[:n], nil
}

// looksLikeText reports whether head is valid UTF-8 with no NUL. When the file
// goes on past head, a character cut in half at the end is not held against
// it.
func looksLikeText(head []byte, truncated bool) bool {
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	if truncated {
		// Drop an incomplete trailing character, which the read may have cut
		// in half, so that it does not fail the UTF-8 check.
		for i := len(head) - 1; i >= 0 && i >= len(head)-utf8.UTFMax; i-- {
			if utf8.RuneStart(head[i]) {
				if !utf8.FullRune(head[i:]) {
					head = head[:i]
				}
				break
			}
		}
	}
	return utf8.Valid(head)
}
