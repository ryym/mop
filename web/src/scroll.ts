// Scroll synchronisation.
//
// The daemon only relays a source line; where that ends up in pixels can only
// be decided here, where the DOM sizes are known.

const DEFAULT_VIEWPORT_RATIO = 0.5;

type Anchor = { line: number; top: number };

function anchors(container: HTMLElement): Anchor[] {
  const found: Anchor[] = [];
  for (const el of container.querySelectorAll<HTMLElement>("[data-source-line]")) {
    const line = Number.parseInt(el.dataset.sourceLine ?? "", 10);
    if (Number.isFinite(line)) {
      found.push({ line, top: el.getBoundingClientRect().top + window.scrollY });
    }
  }
  // Document order is usually line order already, but sorting makes the
  // interpolation below independent of that assumption.
  found.sort((a, b) => a.line - b.line);
  return found;
}

export function scrollToLine(
  container: HTMLElement,
  line: number,
  ratio?: number | null,
): void {
  const r = typeof ratio === "number" ? ratio : DEFAULT_VIEWPORT_RATIO;
  const list = anchors(container);
  if (list.length === 0) return;

  let prev: Anchor | null = null;
  let next: Anchor | null = null;
  for (const a of list) {
    if (a.line <= line) {
      prev = a;
    } else {
      next = a;
      break;
    }
  }

  let y: number;
  if (prev && prev.line === line) {
    y = prev.top;
  } else if (prev && next) {
    // No element starts exactly at this line (a code block, a blank line...),
    // so the position is interpolated between the surrounding elements.
    const t = (line - prev.line) / (next.line - prev.line);
    y = prev.top + (next.top - prev.top) * t;
  } else {
    y = (prev ?? next!).top;
  }

  window.scrollTo({ top: Math.max(0, y - window.innerHeight * r) });
}
