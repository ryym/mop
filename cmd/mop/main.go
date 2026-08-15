// Command mop previews Markdown files in the browser.
//
// The same binary is both the CLI and the daemon; `mop daemon start` is what
// turns it into the server.
package main

import (
	"os"

	"github.com/ryym/mop/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
