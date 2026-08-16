// Package web holds the assets embedded into the binary: the page template
// and the bundled JS/CSS. Nothing is loaded from disk or a CDN at run time.
//
// dist/ is a build artifact (`make web`) and is not committed, so the
// directory keeps a .gitkeep to let `go build` succeed on a fresh checkout.
// The binary is then useless until the bundle is built, which is why
// `go install` is not supported.
package web

import (
	"embed"
	_ "embed"
	"errors"
	"io/fs"
)

// all: is needed so the committed .gitkeep counts as a match on a checkout
// where the bundle has not been built yet.
//
//go:embed all:dist
var Dist embed.FS

//go:embed page.html
var PageTemplate string

// CheckBundle reports whether the frontend bundle made it into the binary.
//
// The embed pattern above matches the directory even when it holds nothing but
// .gitkeep, so a binary built without running the bundler looks fine until the
// preview page comes up blank. Failing at startup, with the command to run,
// beats debugging that in the browser.
func CheckBundle() error {
	if _, err := fs.Stat(Dist, "dist/mop.js"); err != nil {
		return errors.New("the frontend bundle is missing from this binary; build with `make build`")
	}
	return nil
}
