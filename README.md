# mop

Live Markdown preview in the browser using [github-markdown-css], driven from the command line.

`mop open README.md` opens a preview that follows the file as you edit it. An
editor can push unsaved buffer contents and the current scroll position, so the
preview stays in step with what you are looking at.

[github-markdown-css]: https://github.com/sindresorhus/github-markdown-css

## Build

There are no prebuilt binaries yet, so mop is built from source. Go and
[Bun](https://bun.sh/) are needed for that, and for nothing else: the frontend
bundle is embedded in the binary, which has no runtime dependencies of its own.

The browser does the Markdown rendering, so that bundle must be built before Go
embeds it. **`go install` is not supported**, because the bundle is generated
output and is not committed: `go install` builds from the module source alone
and never runs Bun.

```sh
make build                # bun install -> bun build -> go build
make VERSION=0.1.0 build  # stamp a version into the binary
```

The result is `bin/mop`: a single binary, plus a browser to view the preview in.
Linux and macOS only; Windows is not supported yet.

## Usage

```sh
# Open a preview. A daemon is started if none is running.
mop open README.md

# Move the preview to a source line.
mop scroll README.md --line 42
mop scroll README.md --line 42 --viewport-ratio 0.35

# Preview content that has not been saved yet.
cat README.md | mop update README.md --line 42

# Inspect and clean up.
mop list
mop close README.md
```

Ports and URLs never need to be typed. The daemon shuts itself down after ten
minutes with no browser connected, and it can also be managed explicitly:

```sh
mop daemon start [--port 7654] [--foreground]
mop daemon stop
```

## Vim / Neovim plugin

[ryym/mop.vim](https://github.com/ryym/mop.vim)

## Test

```sh
make test   # go test ./... and bun test
```

## Documentation

- [docs/architecture.md](docs/architecture.md) — how the CLI, daemon and
  browser divide the work, and why.
- [docs/frontend.md](docs/frontend.md) — the preview page: rendering, DOM
  patching and scroll synchronisation.

## Current limitations

- Windows is not supported (the daemon is detached with `setsid`).
