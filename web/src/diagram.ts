// Mermaid diagrams.
//
// markdown-it turns a ```mermaid fence into <pre class="mermaid">source</pre>
// (see markdown.ts); this module draws those. Mermaid is imported lazily, so a
// document without diagrams never loads it: it is several times the size of
// everything else in the bundle.

let loader: Promise<typeof import("mermaid").default> | null = null;

function loadMermaid(): Promise<typeof import("mermaid").default> {
  if (!loader) {
    loader = import("mermaid").then(({ default: mermaid }) => {
      mermaid.initialize({
        startOnLoad: false,
        // Theme is chosen once, at load. Following a theme switch would mean
        // re-drawing every diagram, which is not worth it here.
        theme: window.matchMedia?.("(prefers-color-scheme: dark)").matches
          ? "dark"
          : "default",
      });
      return mermaid;
    });
  }
  return loader;
}

let seq = 0;

/**
 * Draws every diagram that is not drawn yet. Elements are marked so that the
 * next DOM patch leaves the drawn <svg> alone (see patch.ts): without that,
 * the diagram would be rolled back to its source text on every refresh.
 */
export async function drawDiagrams(container: HTMLElement): Promise<void> {
  const targets = Array.from(
    container.querySelectorAll<HTMLElement>("pre.mermaid"),
  ).filter((el) => el.dataset.mopRendered !== "1");
  if (targets.length === 0) return;

  const mermaid = await loadMermaid();

  for (const el of targets) {
    const source = el.textContent ?? "";
    try {
      const { svg } = await mermaid.render(`mop-mermaid-${seq++}`, source);
      // Mermaid sanitizes its own output (securityLevel defaults to strict).
      el.innerHTML = svg;
      el.classList.remove("mop-diagram-error");
    } catch (err) {
      // A diagram in progress is usually a broken diagram. Keep showing the
      // source rather than blanking the block, and mark it so it is not
      // retried until the source changes.
      el.classList.add("mop-diagram-error");
      el.title = String(err);
    }
    el.dataset.mopSource = source;
    el.dataset.mopRendered = "1";
  }
}
