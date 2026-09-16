// Syntax highlighting with shiki.
//
// Decisions the design docs left open:
//
//   - The JavaScript RegExp engine is used instead of the oniguruma WASM
//     build. It keeps the bundle a single JS file with no WASM blob to embed
//     or fetch, which matters because everything ships inside the Go binary.
//   - Languages are loaded lazily, one dynamic import per grammar, instead of
//     bundled up front. The supported set below is broad (picked from
//     @shikijs/langs' exports, skipping niche/DSL-specific grammars), and
//     loading all of it eagerly made the first render noticeably slower for
//     documents that only ever use one or two languages.
import { createHighlighterCore, type HighlighterCore } from "shiki/core";
import { createJavaScriptRegexEngine } from "shiki/engine/javascript";
import githubLight from "@shikijs/themes/github-light";
import githubDark from "@shikijs/themes/github-dark";

// Keyed by the canonical shiki language id (what markdown.ts resolves a fence
// info string to). Each loader is a separate dynamic import so an unused
// language never ends up in a browser's request at all.
const LANG_LOADERS: Record<string, () => Promise<{ default: unknown }>> = {
  astro: () => import("@shikijs/langs/astro"),
  c: () => import("@shikijs/langs/c"),
  clojure: () => import("@shikijs/langs/clojure"),
  cpp: () => import("@shikijs/langs/cpp"),
  csharp: () => import("@shikijs/langs/csharp"),
  css: () => import("@shikijs/langs/css"),
  csv: () => import("@shikijs/langs/csv"),
  dart: () => import("@shikijs/langs/dart"),
  diff: () => import("@shikijs/langs/diff"),
  dockerfile: () => import("@shikijs/langs/dockerfile"),
  elixir: () => import("@shikijs/langs/elixir"),
  erlang: () => import("@shikijs/langs/erlang"),
  "git-commit": () => import("@shikijs/langs/git-commit"),
  "git-rebase": () => import("@shikijs/langs/git-rebase"),
  glsl: () => import("@shikijs/langs/glsl"),
  go: () => import("@shikijs/langs/go"),
  graphql: () => import("@shikijs/langs/graphql"),
  groovy: () => import("@shikijs/langs/groovy"),
  haskell: () => import("@shikijs/langs/haskell"),
  html: () => import("@shikijs/langs/html"),
  ini: () => import("@shikijs/langs/ini"),
  java: () => import("@shikijs/langs/java"),
  javascript: () => import("@shikijs/langs/javascript"),
  json: () => import("@shikijs/langs/json"),
  json5: () => import("@shikijs/langs/json5"),
  jsonc: () => import("@shikijs/langs/jsonc"),
  jsx: () => import("@shikijs/langs/jsx"),
  kotlin: () => import("@shikijs/langs/kotlin"),
  latex: () => import("@shikijs/langs/latex"),
  less: () => import("@shikijs/langs/less"),
  lua: () => import("@shikijs/langs/lua"),
  makefile: () => import("@shikijs/langs/makefile"),
  markdown: () => import("@shikijs/langs/markdown"),
  nginx: () => import("@shikijs/langs/nginx"),
  "objective-c": () => import("@shikijs/langs/objective-c"),
  "objective-cpp": () => import("@shikijs/langs/objective-cpp"),
  perl: () => import("@shikijs/langs/perl"),
  php: () => import("@shikijs/langs/php"),
  properties: () => import("@shikijs/langs/properties"),
  protobuf: () => import("@shikijs/langs/protobuf"),
  python: () => import("@shikijs/langs/python"),
  r: () => import("@shikijs/langs/r"),
  regex: () => import("@shikijs/langs/regex"),
  ruby: () => import("@shikijs/langs/ruby"),
  rust: () => import("@shikijs/langs/rust"),
  scala: () => import("@shikijs/langs/scala"),
  scss: () => import("@shikijs/langs/scss"),
  shellscript: () => import("@shikijs/langs/shellscript"),
  sql: () => import("@shikijs/langs/sql"),
  "ssh-config": () => import("@shikijs/langs/ssh-config"),
  svelte: () => import("@shikijs/langs/svelte"),
  swift: () => import("@shikijs/langs/swift"),
  terraform: () => import("@shikijs/langs/terraform"),
  toml: () => import("@shikijs/langs/toml"),
  tsx: () => import("@shikijs/langs/tsx"),
  typescript: () => import("@shikijs/langs/typescript"),
  vim: () => import("@shikijs/langs/vim"),
  vue: () => import("@shikijs/langs/vue"),
  wasm: () => import("@shikijs/langs/wasm"),
  xml: () => import("@shikijs/langs/xml"),
  yaml: () => import("@shikijs/langs/yaml"),
};

export type Highlighter = {
  /**
   * Loads whatever languages in `langs` are supported and not loaded yet.
   * Call this before `render` for a given language; an unloaded language
   * makes `render` fall back to null rather than trigger a load itself, so
   * that `render` can stay synchronous for the DOM-patching code in main.ts.
   */
  loadLanguages(langs: string[]): Promise<void>;
  /** Returns highlighted `<pre>` HTML, or null when the language is unknown. */
  render(code: string, lang: string): string | null;
};

export async function createHighlighter(): Promise<Highlighter> {
  const core: HighlighterCore = await createHighlighterCore({
    themes: [githubLight, githubDark],
    langs: [],
    engine: createJavaScriptRegexEngine(),
  });

  const loaded = new Set(core.getLoadedLanguages());

  return {
    async loadLanguages(langs) {
      const toLoad = [...new Set(langs)].filter(
        (lang) => !loaded.has(lang) && lang in LANG_LOADERS,
      );
      await Promise.all(
        toLoad.map(async (lang) => {
          const mod = await LANG_LOADERS[lang]!();
          await core.loadLanguage(mod.default as Parameters<typeof core.loadLanguage>[0]);
          loaded.add(lang);
        }),
      );
    },
    render(code, lang) {
      if (!lang || !loaded.has(lang)) return null;
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
