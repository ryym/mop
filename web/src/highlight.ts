// Syntax highlighting with shiki.
//
// Two decisions the design docs left open are made here:
//
//   - The JavaScript RegExp engine is used instead of the oniguruma WASM
//     build. It keeps the bundle a single JS file with no WASM blob to embed
//     or fetch, which matters because everything ships inside the Go binary.
//   - The initial language set is javascript / rust / shell only. Anything
//     else falls back to a plain, unhighlighted code block.
import { createHighlighterCore, type HighlighterCore } from "shiki/core";
import { createJavaScriptRegexEngine } from "shiki/engine/javascript";
import githubLight from "@shikijs/themes/github-light";
import githubDark from "@shikijs/themes/github-dark";
import langJavaScript from "@shikijs/langs/javascript";
import langRust from "@shikijs/langs/rust";
import langShell from "@shikijs/langs/shellscript";

export type Highlighter = {
  /** Returns highlighted `<pre>` HTML, or null when the language is unknown. */
  render(code: string, lang: string): string | null;
};

// Languages are loaded up front so that highlighting is synchronous from then
// on. An async highlight would make the page flicker as blocks get colored
// one after another.
export async function createHighlighter(): Promise<Highlighter> {
  const core: HighlighterCore = await createHighlighterCore({
    themes: [githubLight, githubDark],
    langs: [langJavaScript, langRust, langShell],
    engine: createJavaScriptRegexEngine(),
  });

  const known = new Set(core.getLoadedLanguages());

  return {
    render(code, lang) {
      if (!lang || !known.has(lang)) return null;
      return core.codeToHtml(code, {
        lang,
        themes: { light: "github-light", dark: "github-dark" },
        // Emit both themes as CSS variables and let the stylesheet pick one
        // per prefers-color-scheme, instead of baking one theme in.
        defaultColor: false,
      });
    },
  };
}
