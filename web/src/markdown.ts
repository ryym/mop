// Markdown to HTML. This is the only place in the project that understands
// Markdown at all; the daemon just moves the text around.
import MarkdownIt from "markdown-it";
import taskLists from "markdown-it-task-lists";
import { addFrontmatter, FRONTMATTER_TOKEN_TYPE } from "./frontmatter";
import type { Highlighter } from "./highlight";
import { escapeHtml } from "./html";

// Block level tokens that get a data-source-line attribute. The frontend spec
// limits this to headings, paragraphs, list items and tables: they are enough
// to interpolate any line in between, and tagging everything would bloat the
// diff morphdom has to reconcile.
const SOURCE_LINE_RULES = ["heading_open", "paragraph_open", "list_item_open", "table_open"];

// Names that map onto shiki's canonical language ids.
const LANG_ALIASES: Record<string, string> = {
  js: "javascript",
  mjs: "javascript",
  cjs: "javascript",
  node: "javascript",
  rs: "rust",
  sh: "shellscript",
  bash: "shellscript",
  zsh: "shellscript",
  shell: "shellscript",
  console: "shellscript",
};

// Resolves a fence info string's first word to the shiki language id the
// highlighter would be asked to render.
function resolveLang(info: string): string {
  const lang = info.trim().split(/\s+/)[0]?.toLowerCase() ?? "";
  return LANG_ALIASES[lang] ?? lang;
}

// Languages used by the fences and the frontmatter in `content`, deduplicated,
// so that only the grammars a document actually needs get loaded.
export function collectLanguages(md: MarkdownIt, content: string): string[] {
  const tokens = md.parse(content, {});
  const langs = new Set<string>();
  for (const token of tokens) {
    if (token.type === "fence") {
      const lang = resolveLang(token.info);
      if (lang && lang !== "mermaid") langs.add(lang);
    } else if (token.type === FRONTMATTER_TOKEN_TYPE) {
      // token.info is already a canonical shiki id; see frontmatter.ts.
      langs.add(token.info);
    }
  }
  return [...langs];
}

export function createMarkdown(highlighter: Highlighter, fileEndpoint: string): MarkdownIt {
  const md = new MarkdownIt({
    // Raw HTML in the source is never turned into HTML, and that is what makes
    // a separate sanitizer unnecessary. Changing this flag is a change of the
    // sanitizing policy itself, not a rendering tweak.
    html: false,
    linkify: true,
    typographer: false,
    highlight(code, info) {
      const lang = resolveLang(info);
      // Diagram fences are handed to the browser as-is; see main.ts.
      if (lang === "mermaid") {
        return `<pre class="mermaid">${escapeHtml(code)}</pre>`;
      }
      // An unknown or not-yet-loaded language is not an error: markdown-it
      // falls back to its own escaped <pre><code> when this returns "".
      return highlighter.render(code, lang) ?? "";
    },
  });

  // Only link text that carries a scheme.
  // Fuzzy matching accepts any two letter TLD, so a bare "README.md" would
  // become a link to http://README.md (.md is Moldova's TLD).
  md.linkify.set({ fuzzyLink: false });

  md.use(taskLists, { label: true });
  addSourceLines(md);
  addFileLinks(md, fileEndpoint);
  addFrontmatter(md, highlighter);
  return md;
}

// addFileLinks points relative links and image sources at the daemon's file
// endpoint. The page lives at /doc/<id>, so a bare "img.png" would resolve to
// the non-existent /doc/img.png instead of the file next to the document.
function addFileLinks(md: MarkdownIt, fileEndpoint: string): void {
  for (const [rule, attr] of [
    ["image", "src"],
    ["link_open", "href"],
  ] as const) {
    const original =
      md.renderer.rules[rule] ??
      ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));
    md.renderer.rules[rule] = (tokens, idx, options, env, self) => {
      const token = tokens[idx]!;
      const url = token.attrGet(attr);
      const rewritten = url === null ? null : toFileUrl(url, fileEndpoint);
      if (rewritten !== null) token.attrSet(attr, rewritten);
      return original(tokens, idx, options, env, self);
    };
  }
}

// toFileUrl returns the file endpoint URL for a relative URL, or null to leave
// the URL as it is.
function toFileUrl(url: string, fileEndpoint: string): string | null {
  if (!isRelative(url)) return null;
  // Split before decoding, so that an encoded "#" or "?" in a file name stays
  // part of the path. The fragment is kept for the browser; the query means
  // nothing to a local file and is dropped.
  const hashAt = url.indexOf("#");
  const fragment = hashAt < 0 ? "" : url.slice(hashAt);
  const path = (hashAt < 0 ? url : url.slice(0, hashAt)).split("?")[0]!;
  if (path === "") return null;
  // markdown-it has already percent-encoded the URL, so it is decoded first
  // to avoid encoding it twice. A malformed escape is left for the browser.
  let decoded: string;
  try {
    decoded = decodeURIComponent(path);
  } catch {
    return null;
  }
  // The path goes in the query: browsers collapse ".." in a URL path, even
  // percent-encoded, which would lose "../img.png".
  return `${fileEndpoint}?path=${encodeURIComponent(decoded)}${fragment}`;
}

// isRelative reports whether url is a path relative to the document, as
// opposed to an absolute path, an in-page anchor or an external URL.
function isRelative(url: string): boolean {
  if (url === "" || url.startsWith("/") || url.startsWith("#")) return false;
  // Exclude anything with a scheme (http:, data:, file:) or protocol relative.
  return !/^[a-z][a-z0-9+.-]*:/i.test(url) && !url.startsWith("//");
}

// addSourceLines wraps the renderer rules so block elements carry the source
// line they start at. token.map is 0 based; the whole tool speaks 1 based
// lines, so it is converted here rather than at any later boundary.
function addSourceLines(md: MarkdownIt): void {
  for (const rule of SOURCE_LINE_RULES) {
    const original =
      md.renderer.rules[rule] ??
      ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));
    md.renderer.rules[rule] = (tokens, idx, options, env, self) => {
      const token = tokens[idx]!;
      if (token.map) {
        token.attrSet("data-source-line", String(token.map[0]! + 1));
      }
      return original(tokens, idx, options, env, self);
    };
  }
}
