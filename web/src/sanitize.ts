// Sanitizing of rendered HTML.
//
// Everything markdown-it renders passes through here before it reaches the
// page. The allowlist follows what GitHub renders, plus what mop's own output
// needs; DOMPurify's default allowlist is not used, as it lets through forms
// and buttons.
import DOMPurify, { type Config, type WindowLike } from "dompurify";
import { toFileUrl } from "./fileurl";

const ALLOWED_TAGS = [
  // Headings, paragraphs and text.
  ...["h1", "h2", "h3", "h4", "h5", "h6", "p", "br", "hr", "div", "span"],
  ...["b", "i", "strong", "em", "s", "strike", "del", "ins", "sub", "sup", "small", "mark"],
  ...["abbr", "cite", "dfn", "kbd", "q", "samp", "var", "tt", "time", "wbr", "bdo"],
  ...["ruby", "rt", "rp"],
  // Lists and quotes.
  ...["ul", "ol", "li", "dl", "dt", "dd", "blockquote"],
  // Tables.
  ...["table", "caption", "thead", "tbody", "tfoot", "tr", "th", "td"],
  // Code, links and images.
  ...["pre", "code", "a", "img", "picture", "source", "figure", "figcaption"],
  ...["details", "summary"],
  // Task lists: a checkbox, wrapped in a label.
  ...["input", "label"],
];

// Attributes allowed on any of the tags above. aria-* attributes are allowed
// by DOMPurify on its own.
const COMMON_ATTRS = [
  ...["align", "alt", "title", "width", "height", "lang", "dir", "id", "name", "class"],
  ...["colspan", "rowspan", "scope", "start", "open", "tabindex", "datetime"],
  // Scroll sync anchors (see scroll.ts). A forged one only misplaces the
  // scroll position.
  "data-source-line",
  // Restricted to highlighted code by a hook below.
  "style",
];

// Attributes allowed only on the given tags.
const TAG_ATTRS: Record<string, string[]> = {
  a: ["href"],
  img: ["src", "srcset"],
  source: ["srcset", "media", "type"],
  input: ["type", "checked", "disabled"],
  label: ["for"],
};

// TAG_ONLY_ATTRS maps each attribute in TAG_ATTRS to the tags it is allowed on.
const TAG_ONLY_ATTRS = new Map<string, Set<string>>();
for (const [tag, attrs] of Object.entries(TAG_ATTRS)) {
  for (const attr of attrs) {
    if (!TAG_ONLY_ATTRS.has(attr)) TAG_ONLY_ATTRS.set(attr, new Set());
    TAG_ONLY_ATTRS.get(attr)!.add(tag);
  }
}

const CONFIG: Config & { RETURN_DOM_FRAGMENT: true } = {
  ALLOWED_TAGS,
  ALLOWED_ATTR: [...COMMON_ATTRS, ...TAG_ONLY_ATTRS.keys()],
  ALLOW_DATA_ATTR: false,
  // http(s) and mailto URLs, and anything without a scheme. DOMPurify lets
  // data: URLs through on <img src> separately; markdown-it already renders
  // data:image URLs from Markdown images.
  ALLOWED_URI_REGEXP: /^(?:(?:https?|mailto):|[^a-z]|[a-z+.-]+(?:[^a-z+.\-:]|$))/i,
  // Parsed in DOMPurify's inert document, so nothing in the HTML runs or loads
  // before it is sanitized, and it is not parsed a second time by the caller.
  RETURN_DOM_FRAGMENT: true,
};

export type Sanitizer = (html: string) => DocumentFragment;

/**
 * Creates a sanitizer bound to `window`, pointing relative URLs at
 * `fileEndpoint`. Tests pass a jsdom window; the page uses its own.
 */
export function createSanitizer(window: WindowLike, fileEndpoint: string): Sanitizer {
  const purify = DOMPurify(window);

  purify.addHook("uponSanitizeAttribute", (node, data) => {
    const tag = node.nodeName.toLowerCase();
    const tags = TAG_ONLY_ATTRS.get(data.attrName);
    if (tags && !tags.has(tag)) {
      data.keepAttr = false;
    } else if (data.attrName === "style") {
      // shiki colours code with inline styles and has no other way to. Outside
      // of it, a style could only be used to make the page lie about itself.
      // Writing class="shiki" by hand gets past this, to the same effect.
      data.keepAttr = node.closest("pre.shiki") !== null;
    } else if (data.attrName === "id" && data.attrValue.startsWith("mop-")) {
      // The page's own elements and drawn diagrams are looked up by these ids.
      data.keepAttr = false;
    }
  });

  purify.addHook("afterSanitizeAttributes", (node) => {
    // Keep checkbox inputs and drop any other: a checkbox is the only input
    // Markdown can render, through the "- [x]" task list syntax.
    if (node.nodeName === "INPUT" && node.getAttribute("type") !== "checkbox") {
      node.remove();
      return;
    }
    // Point relative URLs written in raw HTML at the file endpoint. Markdown
    // links and images have been rewritten by markdown.ts already, and no
    // longer look relative.
    for (const attr of ["href", "src"]) {
      const url = node.getAttribute(attr);
      const rewritten = url === null ? null : toFileUrl(url, fileEndpoint);
      if (rewritten !== null) node.setAttribute(attr, rewritten);
    }
    const srcset = node.getAttribute("srcset");
    if (srcset !== null) node.setAttribute("srcset", rewriteSrcset(srcset, fileEndpoint));
  });

  return (html) => purify.sanitize(html, CONFIG);
}

// rewriteSrcset rewrites each URL in a srcset, a comma separated list of a URL
// followed by an optional width or density descriptor. A URL containing a
// comma is not told apart from the separator; such names are rare enough.
function rewriteSrcset(srcset: string, fileEndpoint: string): string {
  return srcset
    .split(",")
    .map((candidate) => {
      const [url = "", ...descriptor] = candidate.trim().split(/\s+/);
      const rewritten = toFileUrl(url, fileEndpoint) ?? url;
      return [rewritten, ...descriptor].join(" ");
    })
    .join(", ");
}
