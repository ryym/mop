import { expect, test } from "bun:test";
import { createHighlighter } from "./highlight";
import { createMarkdown } from "./markdown";

const highlighter = await createHighlighter();
await highlighter.loadLanguages(["javascript"]);
const md = createMarkdown(highlighter, "/doc/abc123/file");

test("block elements carry 1 based source lines", () => {
  const html = md.render("# Title\n\ntext\n\n- item\n");
  expect(html).toContain('<h1 data-source-line="1">');
  expect(html).toContain('<p data-source-line="3">');
  expect(html).toContain('<li data-source-line="5">');
});

test("raw HTML in the source is not turned into HTML", () => {
  const html = md.render("<script>alert(1)</script>\n");
  expect(html).not.toContain("<script>");
  expect(html).toContain("&lt;script&gt;");
});

test("a loaded language is highlighted, an unknown one is not", () => {
  expect(md.render("```js\nconst a = 1;\n```\n")).toContain("shiki");
  expect(md.render("```nosuchlang\nx\n```\n")).not.toContain("shiki");
});

test("relative images and links point at the document's file endpoint", () => {
  const html = md.render("![a](img.png) ![b](./sub/x.png) [c](../README.md)\n");
  expect(html).toContain('src="/doc/abc123/file?path=img.png"');
  expect(html).toContain('src="/doc/abc123/file?path=.%2Fsub%2Fx.png"');
  expect(html).toContain('href="/doc/abc123/file?path=..%2FREADME.md"');
});

test("reference style links are rewritten too", () => {
  const html = md.render("[a][ref]\n\n[ref]: other.md\n");
  expect(html).toContain('href="/doc/abc123/file?path=other.md"');
});

test("the fragment stays outside the query and the query is dropped", () => {
  const html = md.render("[a](other.md#sec) ![b](img.png?v=1)\n");
  expect(html).toContain('href="/doc/abc123/file?path=other.md#sec"');
  expect(html).toContain('src="/doc/abc123/file?path=img.png"');
});

test("paths are encoded once, and encoded # stays in the path", () => {
  const html = md.render("![a](画像.png) [b](<my file.md>) [c](a%23b.md)\n");
  expect(html).toContain('src="/doc/abc123/file?path=%E7%94%BB%E5%83%8F.png"');
  expect(html).toContain('href="/doc/abc123/file?path=my%20file.md"');
  expect(html).toContain('href="/doc/abc123/file?path=a%23b.md"');
});

test("absolute URLs, root paths, fragments and data URLs are left alone", () => {
  const html = md.render(
    "![a](https://example.com/x.png) ![b](/x.png) [c](#sec) ![d](data:image/png;base64,AA) [e](//example.com/)\n",
  );
  expect(html).toContain('src="https://example.com/x.png"');
  expect(html).toContain('src="/x.png"');
  expect(html).toContain('href="#sec"');
  expect(html).toContain('src="data:image/png;base64,AA"');
  expect(html).toContain('href="//example.com/"');
});

test("only text with a scheme is linkified", () => {
  const html = md.render("see README.md, www.example.com and https://example.com\n");
  expect(html).not.toContain('href="http://README.md"');
  expect(html).not.toContain('href="http://www.example.com"');
  expect(html).toContain('href="https://example.com"');
});

test("mermaid fences are left for the browser to draw", () => {
  expect(md.render("```mermaid\ngraph TD; A-->B;\n```\n")).toContain(
    '<pre class="mermaid">graph TD; A--&gt;B;',
  );
});

test("YAML, TOML and JSON frontmatter at the top of the document is highlighted", async () => {
  const highlighter2 = await createHighlighter();
  await highlighter2.loadLanguages(["yaml", "toml", "json"]);
  const md2 = createMarkdown(highlighter2, "/doc/abc123/file");

  const yaml = md2.render("---\ntitle: Hello\n---\n\n# Body\n");
  expect(yaml).toContain('<details class="mop-frontmatter" open data-source-line="1">');
  expect(yaml).toContain("<summary>Metadata</summary>");
  expect(yaml).toContain("shiki");
  expect(yaml).toContain("<h1");

  const toml = md2.render('+++\ntitle = "Hello"\n+++\n\n# Body\n');
  expect(toml).toContain('<details class="mop-frontmatter" open data-source-line="1">');
  expect(toml).toContain("shiki");

  const json = md2.render(';;;\n{ "title": "Hello" }\n;;;\n\n# Body\n');
  expect(json).toContain('<details class="mop-frontmatter" open data-source-line="1">');
  expect(json).toContain("shiki");
});

test("a document without frontmatter is unaffected", () => {
  const html = md.render("# Title\n\ntext\n");
  expect(html).not.toContain("mop-frontmatter");
});

test("a --- block that is not at the very top, or has no closing line, is left as plain Markdown", () => {
  // The classic "thematic break in the middle of a document" case.
  const middle = md.render("# Title\n\n---\n\ntext\n");
  expect(middle).not.toContain("mop-frontmatter");
  expect(middle).toContain("<hr>");

  // Opens like frontmatter but is never closed.
  const unclosed = md.render("---\ntitle: Hello\n\n# Body\n");
  expect(unclosed).not.toContain("mop-frontmatter");
});

test("source lines after a frontmatter block still match the source", () => {
  const html = md.render("---\ntitle: Hello\n---\n\n# Body\n");
  expect(html).toContain('<h1 data-source-line="5">');
});
