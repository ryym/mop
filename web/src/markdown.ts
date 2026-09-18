// Markdown to HTML. This is the only place in the project that understands
// Markdown at all; the daemon just moves the text around.
import MarkdownIt from "markdown-it";
import taskLists from "markdown-it-task-lists";
import type StateBlock from "markdown-it/lib/rules_block/state_block.mjs";
import type { Highlighter } from "./highlight";

// Block level tokens that get a data-source-line attribute. The frontend spec
// limits this to headings, paragraphs, list items and tables: they are enough
// to interpolate any line in between, and tagging everything would bloat the
// diff morphdom has to reconcile.
const SOURCE_LINE_RULES = [
  "heading_open",
  "paragraph_open",
  "list_item_open",
  "table_open",
];

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

// Languages used by the fences and the frontmatter in `content`,
// deduplicated. Used to load only the shiki grammars a document actually
// needs before rendering it; see main.ts.
export function collectLanguages(md: MarkdownIt, content: string): string[] {
  const tokens = md.parse(content, {});
  const langs = new Set<string>();
  for (const token of tokens) {
    if (token.type === "fence") {
      const lang = resolveLang(token.info);
      if (lang && lang !== "mermaid") langs.add(lang);
    } else if (token.type === "frontmatter") {
      // token.info is already a canonical shiki id; see frontmatterRule.
      langs.add(token.info);
    }
  }
  return [...langs];
}

// Frontmatter delimiters mapped to the shiki language used to highlight the
// block they open. Only a marker on the document's first line, closed by a
// matching line, is recognized as frontmatter at all; see frontmatterRule.
const FRONTMATTER_MARKERS: Record<string, string> = {
  "---": "yaml",
  "+++": "toml",
  ";;;": "json",
};

// Recognizes a frontmatter block at the very start of the document and
// tokenizes it as a single "frontmatter" token, carrying its language in
// token.info the same way a fence token carries its info string. Registered
// before "hr" so it gets first look at a document-opening "---" line, which
// "hr" would otherwise claim.
//
// Anything that is not exactly this shape — a recognized marker elsewhere in
// the document, or one with no closing line — is left alone and falls
// through to ordinary block parsing (thematic breaks, headings, ...), same
// as before this rule existed.
function frontmatterRule(
  state: StateBlock,
  startLine: number,
  endLine: number,
  silent: boolean,
): boolean {
  if (startLine !== 0) return false;

  const pos = state.bMarks[startLine]! + state.tShift[startLine]!;
  const max = state.eMarks[startLine]!;
  const marker = state.src.slice(pos, max).trim();
  const lang = FRONTMATTER_MARKERS[marker];
  if (!lang) return false;

  let nextLine = startLine + 1;
  let closed = false;
  for (; nextLine < endLine; nextLine++) {
    const p = state.bMarks[nextLine]! + state.tShift[nextLine]!;
    const m = state.eMarks[nextLine]!;
    if (state.src.slice(p, m).trim() === marker) {
      closed = true;
      break;
    }
  }
  if (!closed) return false;
  if (silent) return true;

  const token = state.push("frontmatter", "", 0);
  token.map = [startLine, nextLine + 1];
  token.info = lang;
  token.content = state.getLines(startLine + 1, nextLine, state.blkIndent, false);
  state.line = nextLine + 1;
  return true;
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export function createMarkdown(
  highlighter: Highlighter,
  assetBase: string,
): MarkdownIt {
  const md = new MarkdownIt({
    // Required. Raw HTML in the source is never turned into HTML, and that is
    // what makes a separate sanitizer unnecessary. Changing this flag is a
    // change of the sanitizing policy itself, not a rendering tweak.
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
      // collectLanguages + Highlighter.loadLanguages, called before render
      // in main.ts, is what makes "not yet loaded" the rare case.
      return highlighter.render(code, lang) ?? "";
    },
  });

  md.use(taskLists, { label: true });
  addSourceLines(md);
  addAssetPaths(md, assetBase);
  addFrontmatter(md, highlighter);
  return md;
}

// addFrontmatter wires frontmatterRule into block parsing and renders the
// token it produces through the same highlighter fences use.
//
// The block always starts at line 1, so it gets data-source-line itself
// rather than going through addSourceLines below: that helper assumes a rule
// renders via self.renderToken, which this custom output does not.
function addFrontmatter(md: MarkdownIt, highlighter: Highlighter): void {
  md.block.ruler.before("hr", "frontmatter", frontmatterRule, { alt: [] });

  md.renderer.rules.frontmatter = (tokens, idx) => {
    const token = tokens[idx]!;
    const line = token.map ? token.map[0]! + 1 : 1;
    const html =
      highlighter.render(token.content, token.info) ??
      `<pre>${escapeHtml(token.content)}</pre>`;
    return `<div class="mop-frontmatter" data-source-line="${line}">${html}</div>\n`;
  };
}

// addAssetPaths points relative image sources at the daemon's asset endpoint.
//
// The page lives at /doc/<id>, so a bare "img.png" would resolve to
// /doc/img.png, which is not a route. The file actually sits next to the
// document, and the daemon serves that directory under /doc/<id>/asset/.
//
// Paths that climb out of the document's directory are left alone: the daemon
// serves the base directory only, so rewriting them would just produce a
// different 404.
function addAssetPaths(md: MarkdownIt, assetBase: string): void {
  const original =
    md.renderer.rules.image ??
    ((tokens, idx, options, _env, self) =>
      self.renderToken(tokens, idx, options));

  md.renderer.rules.image = (tokens, idx, options, env, self) => {
    const token = tokens[idx]!;
    const src = token.attrGet("src");
    if (src !== null && isRelativeAsset(src)) {
      token.attrSet("src", assetBase + src.replace(/^\.\//, ""));
    }
    return original(tokens, idx, options, env, self);
  };
}

function isRelativeAsset(src: string): boolean {
  if (src === "" || src.startsWith("/") || src.startsWith("#")) return false;
  if (src.startsWith("../")) return false;
  // Anything with a scheme (http:, data:, file:) or protocol relative.
  return !/^[a-z][a-z0-9+.-]*:/i.test(src) && !src.startsWith("//");
}

// addSourceLines wraps the renderer rules so block elements carry the source
// line they start at. token.map is 0 based; the whole tool speaks 1 based
// lines, so it is converted here rather than at any later boundary.
function addSourceLines(md: MarkdownIt): void {
  for (const rule of SOURCE_LINE_RULES) {
    const original =
      md.renderer.rules[rule] ??
      ((tokens, idx, options, _env, self) =>
        self.renderToken(tokens, idx, options));
    md.renderer.rules[rule] = (tokens, idx, options, env, self) => {
      const token = tokens[idx]!;
      if (token.map) {
        token.attrSet("data-source-line", String(token.map[0]! + 1));
      }
      return original(tokens, idx, options, env, self);
    };
  }
}
