import { beforeEach, expect, test } from "bun:test";
import { JSDOM } from "jsdom";

// patch() works on the page's own document, so jsdom stands in for it.
const { window } = new JSDOM('<main id="mop-content"></main>');
Object.assign(globalThis, {
  document: window.document,
  Element: window.Element,
  HTMLElement: window.HTMLElement,
  HTMLDetailsElement: window.HTMLDetailsElement,
});
const { patch } = await import("./patch");

const container = window.document.getElementById("mop-content") as HTMLElement;

function render(html: string): void {
  patch(container, window.document.createRange().createContextualFragment(html));
}

function states(): string {
  return [...container.querySelectorAll("details")]
    .map((el) => `${el.querySelector("summary")?.textContent}:${el.open ? "open" : "closed"}`)
    .join(" ");
}

beforeEach(() => container.replaceChildren());

test("a <details> keeps the reader's state across a patch", () => {
  render("<details><summary>A</summary></details>");
  container.querySelector("details")!.open = true;
  render("<details><summary>A</summary><p>edited</p></details>");
  expect(states()).toBe("A:open");
});

test("a <details> keeps the reader's state when blocks are added before it", () => {
  render("<h1>T</h1><p>a</p><details><summary>A</summary></details><p>b</p>");
  const el = container.querySelector("details")!;
  el.open = true;
  render(
    "<h1>T</h1><p>a</p><p>new</p><ul><li>x</li></ul><details><summary>A</summary></details><p>b</p>",
  );
  expect(container.querySelector("details")).toBe(el);
  expect(states()).toBe("A:open");
});

test("elements sharing an id are all replaced", () => {
  render('<p id="a">1</p><p id="a">2</p><p>3</p>');
  render('<p id="a">X</p><p>mid</p><p id="a">Y</p>');
  expect(container.innerHTML).toBe('<p id="a">X</p><p>mid</p><p id="a">Y</p>');
  render("<p>none</p>");
  expect(container.innerHTML).toBe("<p>none</p>");
});

test("a change to open in the source wins over the reader's state", () => {
  render("<details><summary>A</summary></details>");
  container.querySelector("details")!.open = true;
  render("<details><summary>A</summary></details>");
  container.querySelector("details")!.open = false;
  render("<details open><summary>A</summary></details>");
  expect(states()).toBe("A:open");
});
