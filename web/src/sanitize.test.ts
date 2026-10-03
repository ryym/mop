import { expect, test } from "bun:test";
import type { WindowLike } from "dompurify";
import { JSDOM } from "jsdom";
import { createHighlighter } from "./highlight";
import { collectLanguages, createMarkdown } from "./markdown";
import { createSanitizer } from "./sanitize";

const { window } = new JSDOM("");
const sanitizer = createSanitizer(window as unknown as WindowLike, "/doc/abc123/file");

function sanitize(html: string): string {
  const div = window.document.createElement("div");
  div.append(sanitizer(html));
  return div.innerHTML;
}

// The same HTML, parsed and serialized without sanitizing, so that the two
// differ only in what the sanitizer did.
function normalize(html: string): string {
  const div = window.document.createElement("div");
  div.innerHTML = html;
  return div.innerHTML;
}

test("mop's own output passes through unchanged", async () => {
  const highlighter = await createHighlighter();
  const md = createMarkdown(highlighter, "/doc/abc123/file");
  // The fixture covers every construct mop renders: frontmatter, highlighted
  // code, diagrams, task lists, tables, images and links.
  const content = await Bun.file(new URL("../dev/sample.md", import.meta.url)).text();
  await highlighter.loadLanguages(collectLanguages(md, content));
  const extra = "| a | b |\n|:-|-:|\n| 1 | 2 |\n\n![d](data:image/png;base64,AA)\n";
  const html = md.render(`${content}\n\n${extra}`);

  expect(html).toContain('class="shiki');
  expect(html).toContain("task-list-item-checkbox");
  expect(html).toContain('class="mop-frontmatter"');
  expect(sanitize(html)).toBe(normalize(html));
});

test("raw HTML in a document is rendered within the allowlist", async () => {
  const highlighter = await createHighlighter();
  const md = createMarkdown(highlighter, "/doc/abc123/file");
  const html = sanitize(
    md.render(
      '<details>\n<summary>More</summary>\n\nInside <kbd>Ctrl</kbd>\n\n</details>\n\n<img src="logo.png" onerror="alert(1)">\n<script>alert(1)</script>\n',
    ),
  );
  expect(html).toContain("<details>\n<summary>More</summary>");
  expect(html).toContain('<p data-source-line="4">Inside <kbd>Ctrl</kbd></p>');
  expect(html).toContain('<img src="/doc/abc123/file?path=logo.png">');
  expect(html).not.toContain("onerror");
  expect(html).not.toContain("script");
});

test("scripts and anything that runs them are removed", () => {
  const payloads = [
    "<script>alert(1)</script>",
    "<img src=x onerror=alert(1)>",
    '<a href="javascript:alert(1)">x</a>',
    '<a href="data:text/html,<script>alert(1)</script>">x</a>',
    '<iframe srcdoc="<script>alert(1)</script>"></iframe>',
    '<object data="x.swf"></object><embed src="x.swf">',
    "<svg><script>alert(1)</script></svg>",
    "<svg><a href=javascript:alert(1)><text>x</text></a></svg>",
    "<math><mtext><table><mglyph><style><img src=x onerror=alert(1)>",
    '<form action="/x"><button>x</button></form>',
  ];
  for (const payload of payloads) {
    const out = sanitize(payload);
    expect(out).not.toMatch(/<script|onerror|javascript:|<iframe|<object|<embed|<svg|<math/i);
    expect(out).not.toMatch(/<form|<button|data:text/i);
  }
});

test("tags that change the page around the document are removed", () => {
  const out = sanitize(
    '<meta http-equiv="refresh" content="0;url=https://example.com"><base href="https://example.com/"><link rel="stylesheet" href="x.css"><style>body{display:none}</style>',
  );
  expect(out).toBe("");
});

test("style is kept only inside highlighted code", () => {
  expect(sanitize('<p style="color:red">x</p>')).toBe("<p>x</p>");
  expect(
    sanitize('<pre class="shiki" style="color:red"><span style="color:blue">x</span></pre>'),
  ).toBe('<pre class="shiki" style="color:red"><span style="color:blue">x</span></pre>');
});

test("only checkbox inputs are kept", () => {
  expect(sanitize('<input type="checkbox" checked disabled>')).toBe(
    '<input type="checkbox" checked="" disabled="">',
  );
  expect(sanitize('<input type="text" value="x"><input>')).toBe("");
});

test("tag specific attributes are dropped elsewhere", () => {
  expect(sanitize('<div href="x" src="y" for="z">x</div>')).toBe("<div>x</div>");
});

test("data attributes other than the source line are dropped", () => {
  expect(sanitize('<p data-source-line="3" data-mop-rendered="1">x</p>')).toBe(
    '<p data-source-line="3">x</p>',
  );
});

test("ids that could collide with the page's own elements are dropped", () => {
  expect(sanitize('<div id="mop-content">x</div><div id="intro">y</div>')).toBe(
    '<div>x</div><div id="intro">y</div>',
  );
});

test("names that clobber document properties are dropped", () => {
  expect(sanitize('<img name="cookie" src="/x.png">')).toBe('<img src="/x.png">');
});

test("HTML that READMEs commonly use is kept", () => {
  const samples = [
    '<p align="center"><img src="/logo.png" width="200" alt="logo"></p>',
    '<picture><source media="(prefers-color-scheme: dark)" srcset="/dark.png"><img src="/light.png"></picture>',
    "<details open><summary>More</summary><p>x</p></details>",
    "<kbd>Ctrl</kbd> H<sub>2</sub>O x<sup>2</sup>",
    '<a href="mailto:a@example.com">a</a> <a href="https://example.com/">b</a>',
    '<img src="data:image/png;base64,AA">',
  ];
  for (const html of samples) {
    expect(sanitize(html)).toBe(normalize(html));
  }
});

test("relative URLs in raw HTML point at the document's file endpoint", () => {
  expect(sanitize('<a href="../README.md#usage">a</a>')).toBe(
    '<a href="/doc/abc123/file?path=..%2FREADME.md#usage">a</a>',
  );
  expect(sanitize('<img src="./logo.png">')).toBe('<img src="/doc/abc123/file?path=.%2Flogo.png">');
  expect(sanitize('<img src="my%20logo.png">')).toBe(
    '<img src="/doc/abc123/file?path=my%20logo.png">',
  );
});

test("each URL in a srcset is rewritten, keeping its descriptor", () => {
  expect(sanitize('<picture><source srcset="dark.png 1x, dark@2x.png 2x"></picture>')).toBe(
    '<picture><source srcset="/doc/abc123/file?path=dark.png 1x, /doc/abc123/file?path=dark%402x.png 2x"></picture>',
  );
  expect(sanitize('<img srcset="https://example.com/a.png 100w, b.png">')).toBe(
    '<img srcset="https://example.com/a.png 100w, /doc/abc123/file?path=b.png">',
  );
});

test("absolute URLs, root paths and fragments in raw HTML are left alone", () => {
  const html = '<a href="https://example.com/">a</a><a href="#sec">b</a><img src="/x.png">';
  expect(sanitize(html)).toBe(html);
});
