# Goal

Make relative URLs in a previewed document resolve to the files they name:

- A link to another Markdown document opens that document in a mop preview.
- An image outside the document's directory (`../img.png`) is displayed.
- A link to any other file (`./script.sh`) opens that file in the browser.

Along with this, mop treats only Markdown files (`.md`, `.markdown`) as
documents. Opening a preview from the CLI and following a link apply the same
rule, so what one can open, the other can too.

# Context

The preview page's URL is not the document's location on disk, so a relative
link such as `./other.md` resolves against the page's URL and 404s. Images
are already redirected to the daemon, but only within the document's own
directory, so `../` paths do not work at all. Following links between
documents, which is how most repositories organise their docs, does not work.

Why the design looks like this:

- **Relative URLs are rewritten when Markdown is rendered**, not on the DOM
  afterwards. The DOM is patched against freshly rendered HTML, so a later
  rewrite would be undone and redone on every update, reloading images each
  time.
- **The frontend rewrites every relative URL the same way**, and the daemon
  decides what the target is by its extension: a document is registered and
  the browser is redirected to its preview; anything else is served as a file.
  This keeps a single definition of "document". Looking at an extension is not
  parsing Markdown, so the daemon stays Markdown-agnostic.
- **The relative path travels in the query string**, not the URL path.
  Browsers collapse `..` segments in a path, even percent-encoded ones.
- **The daemon normalises paths the same way the CLI does.** Otherwise a file
  reached through a symlink would get a different identity and a second
  preview.
- **mop decides the Content-Type of served files itself.** Go's file server
  consults the OS MIME table, which on macOS makes `.ts` and `.sh` files
  download instead of display, and differs between machines. Instead:
  - Common image types are served as images.
  - PDFs are downloaded, because browsers refuse to show them under the
    sandbox that served files are given.
  - Anything else is shown as plain text if it looks like UTF-8, and is
    downloaded otherwise. HTML is shown as source.

Security:

- Any web page can make the browser request the daemon's file URLs. It could
  embed them to probe which files exist, or navigate to one to make the
  daemon register and watch arbitrary documents.
- To stop this, every request that the browser marks as coming from another
  site (`Sec-Fetch-Site` of `cross-site` or `same-site`) is refused, not only
  file requests. The preview pages had smaller leaks of the same kind: a
  foreign page could tell which documents are open by whether their pages
  load, and hold an event stream open to keep the daemon alive. The preview
  itself, the address bar, bookmarks and non-browser clients are allowed; an
  attacker's page can pose as none of them. The cost is that a preview URL
  linked from another site no longer opens.
- As defence in depth, files are served only from the document's git
  repository, or its directory when there is none.
- Served files stay sandboxed, so the wider range does not let an HTML or SVG
  file run scripts on the daemon's origin.

Out of scope:

- **Jumping to a section.** The rewritten URL keeps the `#fragment` of a link
  such as `other.md#section`, but the preview does not give headings an `id`,
  so the browser has nothing to scroll to and the document opens at the top.
  This is true of in-page links (`#section`) today as well.
- **Closing documents opened through links.** Such a document stays
  registered and watched after its tab is closed. It does no harm beyond
  that, and the daemon exits anyway once no browser has been connected for a
  while.
- **Links to directories** (`./docs/`) return 404, rather than, say, opening
  the directory's `README.md`.
- **Link-opened previews after a daemon restart.** A restarted daemon has no
  documents registered, so an open preview tab gets 404 on reload. A document
  opened from the CLI comes back with another `mop open`, but a link-opened
  one is not something the user opened from the CLI in the first place. The
  daemon cannot re-register it from the URL alone either, because the id in
  the URL is a hash of the file path.
