# Goal

Stop a local file served by the daemon from reading arbitrary files through the
control plane. Two independent defences:

- The control plane (`/api/`) rejects any request that carries `Origin` or
  `Sec-Fetch-Site`, i.e. anything coming from a browser.
- Local files under `/doc/{id}/asset/` are served with
  `Content-Security-Policy: sandbox` and `X-Content-Type-Options: nosniff`, so
  they never run as scripts on the daemon's origin.

Either one alone closes the current attack path; both go in as defence in depth.

# Context

The daemon serves files next to a previewed document from its own origin. If a
repository contains a malicious HTML or SVG file and the user opens its asset
URL as a top-level page, its script runs on the daemon's origin. Today
`checkRequest` lets same-origin browser requests through to `/api/`, so that
script can:

1. `POST /api/doc/open` with any absolute path the user can read
   (e.g. `~/.ssh/id_rsa`),
2. fetch the returned `/doc/<id>` page, which embeds the content as JSON,
3. send it elsewhere.

It can also list open documents, push fake content to other previews, or shut
the daemon down. It cannot write files or execute commands.

Previewing a Markdown file from someone else's repository is ordinary use. All
the attacker then needs is for the user to open such an asset URL, for example
by following a link in the document.

Why these particular measures:

- `/api/` is meant for the CLI only; the preview page never calls it. The Go
  HTTP client sends neither header, so the CLI is unaffected.
- `Origin` alone is not enough: browsers may omit it on same-origin GETs, which
  would let `GET /api/docs` through. `Sec-Fetch-Site` is sent on every request
  by current browsers. Its value is not inspected; even `none` (typed into the
  address bar) is rejected, since there is no browser use of `/api/`.
- Relative `<img>` sources in the preview do hit the asset endpoint and receive
  the header, but browsers ignore CSP on subresource responses, so images still
  render. The header only takes effect when an asset URL is opened as a page.
  The preview page and bundle are served from other routes and are untouched.
- Serving HTML/SVG as `text/plain` was rejected because SVG images would stop
  rendering.
- Assets opened as a page lose some behaviour. Both are accepted:
  - HTML and SVG files render, but their scripts do not run and their links do
    not open new windows. This is exactly what the sandbox is for.
  - A PDF no longer renders in the browser's viewer, which refuses to run in a
    sandboxed document. mop does not support PDFs.
- The Host header check (DNS rebinding defence) stays as is.
