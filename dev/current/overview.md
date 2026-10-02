# Goal

Render raw HTML written in Markdown, roughly to the extent GitHub does:

- Common README patterns work: `<p align="center"><img ...></p>`, `<picture>`
  switching images by colour scheme, `<details>` / `<summary>`, `<kbd>`,
  `<sub>` / `<sup>` and so on.
- Relative URLs in raw HTML (`href`, `src`, `srcset`) resolve to the files they
  name, the same way Markdown links and images already do.
- A `<details>` the reader opened or closed stays that way across live updates.

HTML rendering is always on. There is no opt-in or opt-out switch.

# Context

markdown-it runs with `html: false` today, so raw HTML is shown as text. That
flag is the whole sanitizing policy. Many READMEs are written for GitHub and
rely on raw HTML, so they look broken in the preview.

### Threat model

Previewing a Markdown file from an untrusted repository is a normal use, and
its author controls the whole content. The preview page runs on the daemon's
origin, so a script running there could fetch any file the daemon serves for
the document's repository, read other open documents, and send them anywhere.
Removing `<script>` is not enough: event handler attributes, `javascript:`
URLs, `<iframe srcdoc>`, `<object>` and mutation XSS through `<svg>` / `<math>`
all execute script.

### Defences

Two layers:

- **Sanitizing with DOMPurify is the primary defence.** The whole output of
  markdown-it is sanitized against an explicit allowlist of tags and attributes
  based on GitHub's. DOMPurify's default allowlist is not used; it lets through
  forms and buttons.
  - `svg` and `math` are dropped, as GitHub does. Mermaid diagrams are drawn
    after sanitizing, so they are unaffected.
  - `style` is allowed only inside shiki's `<pre class="shiki">`, whose output
    depends on it.
  - URLs are limited to `http:`, `https:`, `mailto:` and relative ones.
  - The sanitized result is a DOM fragment parsed in an inert document, so
    nothing in it fires before it is sanitized.
- **A Content-Security-Policy on the preview page is the fallback** for an
  unknown sanitizer bypass. `script-src 'self'` without `'unsafe-inline'` stops
  inline handlers and `javascript:` URLs, and `connect-src 'self'` stops
  sending data out with `fetch`. `style-src` keeps `'unsafe-inline'`, which
  shiki and mermaid need.
  - `img-src` allows `http:` as well as `https:`. Excluding `http:` would protect
    nothing, because the document's author can point an image at their own
    `https:` server, and it would break images that display today.
  - `Referrer-Policy: no-referrer` keeps the daemon's origin out of requests for
    external images.

The CSP and the sanitizer are introduced while `html: false` is still in
place, and only then is HTML enabled. The first step stands on its own as
hardening, and the second must not change mop's own output.

### Known side effects

Accepted for now:

- While a tag such as `<details>` is being typed and has no closing tag yet,
  the rest of the document ends up inside it until it is closed.
- An HTML block cannot carry `data-source-line`, so scroll sync interpolates
  across it. Markdown paragraphs inside the block still carry one.

### Out of scope

- A switch to disable HTML rendering for untrusted repositories. With the
  sanitizer and the CSP in place it would reduce little risk.
- `<video>` and `<audio>`.
