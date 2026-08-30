# mop

Live Markdown preview in the browser, driven from the command line.

`mop open README.md` opens a preview that follows the file as you edit it. An
editor can push unsaved buffer contents and the current scroll position, so the
preview stays in step with what you are looking at.

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

The result is `./mop`: a single binary, plus a browser to view the preview in.
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

## Vim / Neovim

Source `editor/mop.vim`; no plugin manager is involved.

```vim
source /path/to/mop/editor/mop.vim
let g:mop_command = '/path/to/mop/mop'   " when mop is not on PATH
```

| Command     | Effect                                                 |
| ----------- | ------------------------------------------------------ |
| `:Mop`      | Open a preview of the current buffer and start syncing |
| `:MopClose` | Stop syncing and close the preview                     |

After `:Mop`, buffer edits are sent with `mop update` (**including unsaved
text**) and the window's view with `mop scroll`, so the preview follows where
the window scrolls to. Closing the buffer or quitting Vim runs `mop close`
automatically.

| Variable               | Default | Meaning                                 |
| ---------------------- | ------- | --------------------------------------- |
| `g:mop_command`        | `mop`   | Command to run                          |
| `g:mop_update_delay`   | 150     | Debounce before sending an edit (ms)    |
| `g:mop_scroll_delay`   | 60      | Debounce before sending a scroll (ms)   |
| `g:mop_viewport_ratio` | 0.0     | Where in the viewport to place the line |

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
