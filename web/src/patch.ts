// DOM updates.
//
// The rendered HTML is never assigned with innerHTML on the live tree: a full
// replacement resets the scroll position, reloads images, drops <details>
// state and text selection, and wipes any diagram already drawn. morphdom
// applies it as a patch instead.
import morphdom from "morphdom";

export function patch(container: HTMLElement, content: DocumentFragment): void {
  // morphdom compares two elements, so the new content gets its own container
  // of the same shape. The fragment is sanitized already (see sanitize.ts).
  const next = document.createElement(container.tagName);
  next.id = container.id;
  next.append(content);

  morphdom(container, next, {
    onBeforeElUpdated(fromEl, toEl) {
      // An element already drawn by a diagram renderer holds a <svg> that no
      // longer matches its source text. Patch it only when the source it was
      // drawn from actually changed.
      if (fromEl.dataset?.mopRendered === "1") {
        return fromEl.dataset.mopSource !== toEl.textContent;
      }
      return true;
    },
  });
}
