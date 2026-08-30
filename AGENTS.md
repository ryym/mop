# AGENTS.md

Working notes for agents and humans making changes here.

## Start here

- [README.md](README.md) — what mop is, how to build, run and test it.
- [docs/architecture.md](docs/architecture.md) — CLI / daemon / browser split.
- [docs/frontend.md](docs/frontend.md) — the preview page.

## Invariants

Breaking any of these is a design change, not an implementation detail. Say so
explicitly if you intend to.

- **The daemon does not parse Markdown.** Markdown-aware code belongs in
  `web/src/` only. Go code stores, watches and forwards text.
- **markdown-it runs with `html: false`.** It is the entire sanitizing policy,
  and `patch.ts` writes rendered HTML with `innerHTML` on that basis.
- **Line numbers are 1-based** everywhere outside markdown-it's `token.map`.
- **The DOM is patched, never replaced**.

## Conventions

- English for code, comments and commit messages.
- Comments explain _why_, not what. The existing code is the reference for tone
  and density — match it rather than adding narration.
- `gofmt -w .` (or `make fmt`) before committing Go changes.
- Small, single-purpose commits. All tests pass at every commit.
- `web/dist/` is build output and is never committed.

## Things that bite

- **A running daemon serves the assets from the binary it started from.** After
  a rebuild the CLI restarts it automatically via the build fingerprint; if you
  bypass the CLI, restart it yourself.
- **`go build` alone produces a broken binary** unless the bundle exists. `make`
  checks for it, and so does the daemon on startup.
- **Browser behaviour has no automated coverage.** DOM patching, scroll
  interpolation and diagram redrawing are verified by hand. Change them
  carefully, and check the result in a real preview.

## Documentation

- Rewrite cleanly; never leave stale statements or an edit history behind.
- Do not duplicate what README or `docs/` already says — link to it.
- Docs describe the code as it is. Update them in the same commit as the change.
- **A change that moves a responsibility, an API, a lifecycle rule or a
  behaviour described in `docs/architecture.md` updates that document too, in
  the same commit.** The same goes for `docs/frontend.md` and `web/src/`. A doc
  that contradicts the code is worse than no doc.
