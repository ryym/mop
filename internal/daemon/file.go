package daemon

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// serveFile sends a file linked from a document as it is, sandboxed, with the
// headers setFileHeaders decides for it. A file that cannot be read gets 404.
func serveFile(w http.ResponseWriter, r *http.Request, path string) {
	// Inspect and send the file through one handle, so that the file whose
	// type is decided is the file that is sent.
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	head, err := readHead(f)
	if err == nil {
		_, err = f.Seek(0, io.SeekStart)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	setFileHeaders(w.Header(), path, head, info.Size())
	// Sandbox the file because it comes from whatever repository the user is
	// previewing, yet is served on the daemon's origin. Browsers ignore CSP on
	// subresources, so <img> in the preview is unaffected.
	w.Header().Set("Content-Security-Policy", "sandbox")
	// Disable sniffing so the browser cannot reinterpret a file as a type the
	// sandbox was not expected to cover, or as anything other than the type
	// setFileHeaders chose.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Revalidate rather than serve from cache: an image edited next to the
	// document has to show up in the preview. ServeContent still answers 304
	// while the file is unchanged.
	w.Header().Set("Cache-Control", "no-cache")
	// Pass no name: ServeContent uses it only to guess a type, which is set
	// already.
	http.ServeContent(w, r, "", info.ModTime(), f)
}

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
