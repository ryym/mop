// Markdown to HTML. This is the only place in the project that understands
// Markdown at all; the daemon just moves the text around.
import MarkdownIt from "markdown-it";
import taskLists from "markdown-it-task-lists";
import { addFrontmatter, FRONTMATTER_TOKEN_TYPE } from "./frontmatter";
import type { Highlighter } from "./highlight";
import { toFileUrl } from "./fileurl";
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
    // Raw HTML in the source is rendered as HTML. The output is sanitized
    // before it reaches the page; see sanitize.ts.
    html: true,
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
  alignTableCells(md);
  addFileLinks(md, fileEndpoint);
  addFrontmatter(md, highlighter);
  return md;
}

// alignTableCells renders a column's alignment as an align attribute instead
// of markdown-it's inline style. Inline styles in the rendered HTML are kept
// to highlighted code, which cannot do without them.
function alignTableCells(md: MarkdownIt): void {
  for (const rule of ["th_open", "td_open"]) {
    const original =
      md.renderer.rules[rule] ??
      ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));
    md.renderer.rules[rule] = (tokens, idx, options, env, self) => {
      const token = tokens[idx]!;
      const align = /^text-align:(left|center|right)$/.exec(token.attrGet("style") ?? "")?.[1];
      if (align) {
        token.attrs = token.attrs?.filter(([name]) => name !== "style") ?? null;
        token.attrSet("align", align);
      }
      return original(tokens, idx, options, env, self);
    };
  }
}

// addFileLinks points relative links and image sources at the daemon's file
// endpoint; see fileurl.ts. Ones written in raw HTML are handled by the
// sanitizer.
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
