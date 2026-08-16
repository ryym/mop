// Entry point of the preview page.
//
// Everything the daemon sends is raw Markdown; parsing, highlighting, DOM
// patching and scrolling all happen here.
import { drawDiagrams } from "./diagram";
import { createHighlighter } from "./highlight";
import { createMarkdown } from "./markdown";
import { patch } from "./patch";
import { scrollToLine } from "./scroll";

type RefreshEvent = {
  content: string;
  line?: number | null;
  viewportRatio?: number | null;
};

type ScrollEvent = {
  line: number;
  viewportRatio?: number | null;
};

const container = document.getElementById("mop-content") as HTMLElement;
const statusEl = document.getElementById("mop-status") as HTMLElement;
const docId = document.body.dataset.docId ?? "";

function showStatus(text: string): void {
  statusEl.textContent = text;
  statusEl.hidden = text === "";
}

function readInitialContent(): string {
  const el = document.getElementById("mop-initial");
  if (!el?.textContent) return "";
  try {
    return (JSON.parse(el.textContent) as { content?: string }).content ?? "";
  } catch {
    return "";
  }
}

async function main(): Promise<void> {
  // The highlighter is built once, before the first render, so every render
  // afterwards is a synchronous call and the page never repaints in stages.
  const highlighter = await createHighlighter();
  const md = createMarkdown(highlighter);

  const render = (content: string) => {
    patch(container, md.render(content));
    // Diagrams are drawn after the patch, and their loading is not waited
    // for: the text should not be held back by a diagram library.
    void drawDiagrams(container);
  };

  render(readInitialContent());
  connect(render);
}

function connect(render: (content: string) => void): void {
  const source = new EventSource(`/doc/${docId}/events`);
  let closed = false;

  source.addEventListener("open", () => showStatus(""));

  source.addEventListener("refresh", (ev) => {
    const msg = JSON.parse((ev as MessageEvent<string>).data) as RefreshEvent;
    render(msg.content);
    // A refresh without a line comes from the file watcher. Moving the
    // viewport then would fight with whoever is reading the page.
    if (typeof msg.line === "number") {
      scrollToLine(container, msg.line, msg.viewportRatio);
    }
    showStatus("");
  });

  source.addEventListener("scroll", (ev) => {
    const msg = JSON.parse((ev as MessageEvent<string>).data) as ScrollEvent;
    scrollToLine(container, msg.line, msg.viewportRatio);
  });

  source.addEventListener("close", () => {
    closed = true;
    source.close();
    showStatus("closed");
  });

  source.addEventListener("error", () => {
    if (closed) return;
    // EventSource reconnects by itself, and the reconnect brings a full
    // refresh, so there is no state to repair here.
    showStatus("reconnecting…");
  });
}

void main();
