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
    } else if (token.type === FRONTMATTER_TOKEN_TYPE) {
      // token.info is already a canonical shiki id; see frontmatter.ts.
      langs.add(token.info);
    }
  }
  return [...langs];
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
