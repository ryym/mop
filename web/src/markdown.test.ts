import { expect, test } from "bun:test";
import { createHighlighter } from "./highlight";
import { createMarkdown } from "./markdown";

const md = createMarkdown(await createHighlighter());

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

test("mermaid fences are left for the browser to draw", () => {
  expect(md.render("```mermaid\ngraph TD; A-->B;\n```\n")).toContain(
    '<pre class="mermaid">graph TD; A--&gt;B;',
  );
});
