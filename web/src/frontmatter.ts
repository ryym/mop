// Frontmatter: a "---"/"+++"/";;;" block wrapping metadata at the very start
// of a document. markdown-it has no concept of it, so this module teaches it
// to parse and highlight one.
import type MarkdownIt from "markdown-it";
import type StateBlock from "markdown-it/lib/rules_block/state_block.mjs";
import type { Highlighter } from "./highlight";
import { escapeHtml } from "./html";

// Token type of the tokens addFrontmatter produces.
export const FRONTMATTER_TOKEN_TYPE = "frontmatter";

// Delimiters mapped to the shiki language of the block they open.
const FRONTMATTER_MARKERS: Record<string, string> = {
  "---": "yaml",
  "+++": "toml",
  ";;;": "json",
};

// markdown-it block rule recognizing a frontmatter block: a marker on the
// document's first line, closed by a matching line. It becomes a single
// token carrying its language in token.info, the way a fence token carries
// its info string.
//
// Anything else — a marker elsewhere in the document, or one never closed —
// falls through to ordinary block parsing, as if this rule did not exist.
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

  const token = state.push(FRONTMATTER_TOKEN_TYPE, "", 0);
  token.map = [startLine, nextLine + 1];
  token.info = lang;
  token.content = state.getLines(startLine + 1, nextLine, state.blkIndent, false);
  state.line = nextLine + 1;
  return true;
}

// Makes `md` parse a frontmatter block and render it highlighted, through the
// same highlighter fences use. It renders as a <details>, open by default:
// metadata is worth seeing but not worth pushing the body down permanently.
export function addFrontmatter(md: MarkdownIt, highlighter: Highlighter): void {
  // Register before "hr" so the rule gets first look at a document-opening
  // "---" line, which "hr" would otherwise claim.
  md.block.ruler.before("hr", FRONTMATTER_TOKEN_TYPE, frontmatterRule, { alt: [] });

  md.renderer.rules[FRONTMATTER_TOKEN_TYPE] = (tokens, idx) => {
    const token = tokens[idx]!;
    // Set data-source-line here instead of via markdown.ts's addSourceLines:
    // that helper assumes a rule renders through self.renderToken, which this
    // custom output does not.
    const line = token.map ? token.map[0]! + 1 : 1;
    const html =
      highlighter.render(token.content, token.info) ??
      `<pre>${escapeHtml(token.content)}</pre>`;
    return `<details class="mop-frontmatter" open data-source-line="${line}"><summary>Metadata</summary>${html}</details>\n`;
  };
}
