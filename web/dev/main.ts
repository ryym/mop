// Entry point of the style preview: renders sample.md once, with no daemon,
// no event stream and no build step, so that mop.css can be edited and seen.
//
// The rendering pipeline is the real one (`../src/markdown`), but nothing
// after it is: the document never changes here, so the DOM is written once
// instead of patched, and no scrolling is synchronised.
import { drawDiagrams } from "../src/diagram";
import { createHighlighter } from "../src/highlight";
import { collectLanguages, createMarkdown } from "../src/markdown";
import sample from "./sample.md" with { type: "text" };

const container = document.getElementById("mop-content") as HTMLElement;
const highlighter = await createHighlighter();

// The asset base is unused: sample.md keeps its images self contained, since
// there is no daemon here to serve files next to the document.
const md = createMarkdown(highlighter, "");

// Shiki grammars are loaded lazily, so the ones sample.md needs have to be in
// before the synchronous md.render() below.
await highlighter.loadLanguages(collectLanguages(md, sample));

container.innerHTML = md.render(sample);
void drawDiagrams(container);
