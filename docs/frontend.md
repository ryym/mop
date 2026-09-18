# Frontend

The preview page: what it is responsible for and the rules it follows. For the
surrounding system see [architecture.md](./architecture.md).

## Responsibilities

The page receives raw Markdown and owns everything after that — parsing,
highlighting, updating the DOM, deciding where to scroll, and drawing anything
that only a browser can draw. Each file in `web/src/` states its own role at the
top; those comments are the reference for what lives where.

Everything the page needs is served from the binary; it makes no external
requests. The initial Markdown is embedded in the page itself, so the first
render does not wait for the event stream. Without JavaScript nothing is
displayed at all, which is acceptable for a local tool.

## Sanitizing

markdown-it runs with **`html: false`**, so raw HTML in the source is never
turned into markup. That flag is the entire sanitizing policy — no DOMPurify, no
allowlist — and the rendered HTML is written into the DOM on that basis.

**Changing it is a change of the sanitizing policy, not a rendering tweak.**

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

## Updating the DOM

Rendered HTML is applied as a **patch, never as a replacement**. Replacing the
tree would reset the scroll position, reload images, drop `<details>` state and
text selection, and wipe drawn diagrams.

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
