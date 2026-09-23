# Architecture

The shape of mop and the decisions behind it.
For the preview page see [frontend.md](./frontend.md).

## Overview

mop is a single Go binary that is both the CLI and the preview server (the
daemon). They talk over HTTP, and the daemon pushes to the browser over SSE.

```
[CLI]                        [daemon]                      [browser]
  mop open <file>    --->    registers and watches
  mop update <file>  --->    holds the raw text  --SSE-->   parse, highlight,
  mop scroll --line  --->    pushes text and positions      patch, scroll
```

**The daemon does not understand Markdown.** It stores and forwards raw text;
parsing, highlighting, DOM updates and scroll positioning all happen in the
browser.

| Component | Responsibility                                                    |
| --------- | ----------------------------------------------------------------- |
| CLI       | Resolving paths, ensuring a daemon is up, calling the daemon      |
| Daemon    | Watching files, holding and serving raw text, serving assets      |
| Browser   | Parsing, highlighting, patching the DOM, deciding where to scroll |

The payoff is that Markdown features are a frontend-only concern: mermaid
support, for instance, touched `web/src/` and nothing else.

## Layout

```
cmd/mop/    Entry point; subcommand dispatch only
internal/   The CLI, the daemon and everything they share
web/        The preview page, embedded into the binary
  src/      Browser-side TypeScript and CSS; the only Markdown-aware code
  dist/     Bundle output (generated, not committed)
```

Each package under `internal/` states its own role in a package comment; those
are the reference, not this list.

## HTTP surfaces

The daemon serves two unrelated kinds of traffic on the same port:

- **The control plane, `/api/`.** JSON, spoken by the CLI only. Browser
  requests are refused outright.
- **The preview surface, `/doc/<id>` and `/static/`.** The page, its event
  stream, the bundle, and files sitting next to the document. Spoken by the
  browser only.

### Documents are identified by path

The control plane names a document by its absolute path, and the id in its
preview URL is derived from that path. Nothing else identifies a document: there
are no handles and no sessions, which is what lets every CLI invocation be a
fresh process, and what keeps a preview URL valid across daemon restarts.

Normalising the path is the CLI's job. The daemon treats the path it receives as
the identity as-is.

### Pushing to the browser

Updates flow one way, over the document's event stream; the browser never posts
anything back.

- **The full text is sent every time.** Neither side computes diffs. Avoiding
  the cost of that is the DOM's problem, and the browser solves it by patching.
- **Connecting delivers the current state**, so a browser that reconnects never
  has to work out what it missed.
- **The viewport moves only on an explicit instruction.** An update caused by
  the file changing carries no position, so reading is never interrupted.

## Lifecycle

The daemon is meant to be invisible: started when first needed, gone when it is
not.

- **`mop open` starts a daemon if none is running.** The other commands do not:
  they act on documents a running daemon holds.
- **The port is fixed, with no fallback.** A stable port is what keeps preview
  URLs valid across restarts.
- **A running daemon is found through a state file**, written under
  `$XDG_STATE_HOME/mop` next to the daemon's log.
- **A daemon built from a different binary is replaced.** It serves the assets
  embedded in the binary it started from, so a rebuild would otherwise keep
  being served the old preview page.
- **The daemon exits after a while with no browser connected.** Closing tabs is
  what ends a session, so a forgotten `mop close` leaves no process behind.

## File watching

Editors save in ways that defeat naive file watching — writing a temporary file
and renaming it over the original, or producing a burst of events for one save.
The watcher works around both by watching the containing directory and letting
changes settle before reading.

Disk changes and `mop update` are not ranked against each other: whichever
arrives last wins.

## Security

The daemon is bound to localhost, but any web page can send a browser there, so
it does not treat localhost as trusted:

- **Requests must be addressed to the daemon itself**, not merely routed to it.
  This is the DNS rebinding defence: an attacker's domain can resolve to
  127.0.0.1, but the headers still name the attacker.
- **The control plane refuses any request a browser sends**, even one from the
  daemon's own origin. Opening a document is reading a file, so a script that
  reached `/api/` could read anything the user can. Browsers are recognised by
  `Origin` or `Sec-Fetch-Site`, neither of which the CLI sends.
- **Local files are served only from the document's own directory**, symlinks
  resolved before the decision.
- **Sanitizing rests entirely on the Markdown renderer never emitting raw HTML**
  — see [frontend.md](./frontend.md).

## Build

The frontend bundle is embedded into the binary, so a built mop depends on
nothing at runtime. Two consequences are worth knowing: the daemon serves the
assets it was built with (hence the restart rule above), and Go alone cannot
build the project — see the README.
