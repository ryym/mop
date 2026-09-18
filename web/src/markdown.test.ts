import { expect, test } from "bun:test";
import { createHighlighter } from "./highlight";
import { createMarkdown } from "./markdown";

const highlighter = await createHighlighter();
await highlighter.loadLanguages(["javascript"]);
const md = createMarkdown(highlighter, "/doc/abc123/asset/");

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

test("relative images point at the document's asset endpoint", () => {
  const html = md.render("![a](img.png)\n\n![b](./sub/x.png)\n");
  expect(html).toContain('src="/doc/abc123/asset/img.png"');
  expect(html).toContain('src="/doc/abc123/asset/sub/x.png"');
});

test("absolute and escaping image sources are left alone", () => {
  const html = md.render(
    "![a](https://example.com/x.png)\n\n![b](/x.png)\n\n![c](../x.png)\n\n![d](data:image/png;base64,AA)\n",
  );
  expect(html).toContain('src="https://example.com/x.png"');
  expect(html).toContain('src="/x.png"');
  expect(html).toContain('src="../x.png"');
  expect(html).toContain('src="data:image/png;base64,AA"');
});

test("mermaid fences are left for the browser to draw", () => {
  expect(md.render("```mermaid\ngraph TD; A-->B;\n```\n")).toContain(
    '<pre class="mermaid">graph TD; A--&gt;B;',
  );
});

test("YAML, TOML and JSON frontmatter at the top of the document is highlighted", async () => {
  const highlighter2 = await createHighlighter();
  await highlighter2.loadLanguages(["yaml", "toml", "json"]);
  const md2 = createMarkdown(highlighter2, "/doc/abc123/asset/");

  const yaml = md2.render("---\ntitle: Hello\n---\n\n# Body\n");
  expect(yaml).toContain('<div class="mop-frontmatter" data-source-line="1">');
  expect(yaml).toContain("shiki");
  expect(yaml).toContain("<h1");

  const toml = md2.render('+++\ntitle = "Hello"\n+++\n\n# Body\n');
  expect(toml).toContain('<div class="mop-frontmatter" data-source-line="1">');
  expect(toml).toContain("shiki");

  const json = md2.render(';;;\n{ "title": "Hello" }\n;;;\n\n# Body\n');
  expect(json).toContain('<div class="mop-frontmatter" data-source-line="1">');
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
