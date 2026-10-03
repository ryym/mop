// DOM updates.
//
// The rendered HTML is never assigned with innerHTML on the live tree: a full
// replacement resets the scroll position, reloads images, drops <details>
// state and text selection, and wipes any diagram already drawn. morphdom
// applies it as a patch instead.
import morphdom from "morphdom";

/**
 * Makes `container`'s children match `content`, keeping what the reader or a
 * diagram renderer has changed in the DOM where the source has not.
 */
export function patch(container: HTMLElement, content: DocumentFragment): void {
  // morphdom compares two elements, so the new content gets its own container
  // of the same shape. The fragment is sanitized already (see sanitize.ts).
  const next = document.createElement(container.tagName);
  next.id = container.id;
  next.append(content);
  next.querySelectorAll("details").forEach((el, i) => {
    // Key each <details> by its position among the document's <details>, so
    // that it is matched to the one the reader toggled even when blocks are
    // added or removed before it. Unkeyed, morphdom pairs children by position
    // and tag, and discards an element as soon as another tag takes its place.
    // Adding or removing a <details> before it still shifts the keys, handing
    // its state to its neighbour; nothing in the source identifies a <details>
    // more reliably than its position.
    el.dataset.mopKey = `mop-details-${i}`;
    // Record whether the source opens it, so that a patch can tell the source
    // toggling it from the reader having toggled it.
    el.dataset.mopSourceOpen = el.open ? "1" : "0";
  });

  morphdom(container, next, {
    getNodeKey(node) {
      return node instanceof HTMLDetailsElement ? node.dataset.mopKey : undefined;
    },
    onBeforeElUpdated(fromEl, toEl) {
      // An element already drawn by a diagram renderer holds a <svg> that no
      // longer matches its source text. Patch it only when the source it was
      // drawn from actually changed.
      if (fromEl.dataset?.mopRendered === "1") {
        return fromEl.dataset.mopSource !== toEl.textContent;
      }
      // A <details> keeps the state the reader left it in, unless the source
      // itself was changed to open or close it.
      if (
        fromEl instanceof HTMLDetailsElement &&
        toEl instanceof HTMLDetailsElement &&
        fromEl.dataset.mopSourceOpen === toEl.dataset.mopSourceOpen
      ) {
        toEl.open = fromEl.open;
      }
      return true;
    },
  });
}
