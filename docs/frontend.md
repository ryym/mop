# Frontend

The preview page: what it is responsible for and the rules it follows. For the
surrounding system see [architecture.md](./architecture.md).

## Responsibilities

The page receives raw Markdown and owns everything after that — parsing,
highlighting, updating the DOM, deciding where to scroll, and drawing anything
that only a browser can draw. Each file in `web/src/` states its own role at the
top; those comments are the reference for what lives where.

Everything the page needs is served from the binary; the only external
requests are for images the document itself points at. The initial Markdown is
embedded in the page itself, so the first render does not wait for the event
stream. Without JavaScript nothing is displayed at all, which is acceptable
for a local tool.

## Sanitizing

Raw HTML in the source is rendered, roughly to the extent GitHub renders it.
Documents may come from untrusted repositories, and a script running on the
page could read every file the daemon serves for them, so everything
markdown-it renders is sanitized with DOMPurify before it reaches the DOM
(`sanitize.ts`). The page's CSP (see
[architecture.md](./architecture.md#security)) is the fallback for anything
that gets past it.

The allowlist of tags and attributes follows what GitHub renders, plus what
mop's own output needs. **Widening it is a change of the sanitizing policy, not
a rendering tweak.**

- **`svg` and `math` are not allowed**, as on GitHub; they are the usual ground
  for mutation XSS. Mermaid diagrams are drawn after sanitizing and are not
  affected.
- **`style` is allowed only inside highlighted code.** shiki has no other way
  to colour it. This is also why table alignment is rendered as an `align`
  attribute.
- **URLs are limited to `http:`, `https:`, `mailto:` and relative ones**, plus
  `data:` on `<img src>`.
- **The result is a DOM fragment** parsed in DOMPurify's inert document, so
  nothing in it runs or loads before it is sanitized.

An HTML block cannot carry a source line, so scroll synchronisation
interpolates across it. While a tag such as `<details>` is being typed without
its closing tag, the rest of the document renders inside it.

mop's own output must pass through the sanitizer unchanged; a test holds
`web/dev/sample.md` to that. A construct added to the renderer that the
allowlist does not cover is silently stripped, so it fails there first.

## Highlighting

Highlighting uses shiki, with the JavaScript regex engine rather than the WASM
one: the bundle stays plain JS, with no WASM blob to embed in the binary.

Languages are loaded before the first render, which keeps highlighting
synchronous so the page never repaints in stages. A language that is not loaded
is not an error; it renders as a plain code block.

Themes are emitted for both colour schemes at once and chosen with CSS, rather
than one being baked in at render time.

A YAML, TOML or JSON frontmatter block (`---`, `+++` or `;;;`, opening and
closing the document's first lines) is rendered through this same path,
highlighted as its language rather than parsed as Markdown.

## Source lines

Block elements carry the source line they start at, in a `data-source-line`
attribute. This is the only link between editor positions and the rendered page,
and it is what scroll synchronisation reads.

## Relative links

Relative URLs in links and images are rewritten to the daemon's file endpoint
while rendering; see [architecture.md](./architecture.md#links-between-files).
Markdown links and images are rewritten by markdown-it's renderer; `href`,
`src` and `srcset` written in raw HTML are rewritten by the sanitizer. Doing it
before patching rather than on the DOM matters: a rewrite applied after a patch
would be undone by the next one, reloading images on every update.

## Updating the DOM

Rendered HTML is applied as a **patch, never as a replacement**. Replacing the
tree would reset the scroll position, reload images, drop `<details>` state and
text selection, and wipe drawn diagrams.

A `<details>` keeps the state the reader left it in across patches. Only a
change to its `open` attribute in the source opens or closes it.

## Scrolling

The daemon sends a source line; only the page knows what that is in pixels. The
line is located among the tagged elements, interpolated between them when no
element starts exactly there, and placed in the viewport according to the ratio
the caller asked for.

## Style preview

`web/src/mop.css` is otherwise only visible through a running daemon, which
means a rebuild and a restart for every change. `make dev-style` serves
`web/dev/` instead: `sample.md` — one document holding every construct the
stylesheet has a rule for — rendered through the real pipeline, with the
stylesheet loaded from source and hot reloaded on save.

Only the rendering is real. The document never changes there, so nothing after
markdown-it applies: no event stream, no DOM patching, no scroll
synchronisation. Those are still verified by hand in an actual preview.

`web/dev/index.html` repeats the skeleton of `web/page.html`, so the two move
together: an element or id that changes in the page changes there as well.
