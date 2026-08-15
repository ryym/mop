// Markdown to HTML. This is the only place in the project that understands
// Markdown at all; the daemon just moves the text around.
import MarkdownIt from "markdown-it";
import taskLists from "markdown-it-task-lists";
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

// Names that map onto the languages shiki has loaded.
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

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export function createMarkdown(highlighter: Highlighter): MarkdownIt {
  const md = new MarkdownIt({
    // Required. Raw HTML in the source is never turned into HTML, and that is
    // what makes a separate sanitizer unnecessary. Changing this flag is a
    // change of the sanitizing policy itself, not a rendering tweak.
    html: false,
    linkify: true,
    typographer: false,
    highlight(code, info) {
      const lang = info.trim().split(/\s+/)[0]?.toLowerCase() ?? "";
      // Diagram fences are handed to the browser as-is; see main.ts.
      if (lang === "mermaid") {
        return `<pre class="mermaid">${escapeHtml(code)}</pre>`;
      }
      const resolved = LANG_ALIASES[lang] ?? lang;
      // An unknown language is not an error: markdown-it falls back to its
      // own escaped <pre><code> when this returns an empty string.
      return highlighter.render(code, resolved) ?? "";
    },
  });

  md.use(taskLists, { label: true });
  addSourceLines(md);
  return md;
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
