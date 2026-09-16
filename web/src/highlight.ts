// Syntax highlighting with shiki.
//
// Two decisions the design docs left open are made here:
//
//   - The JavaScript RegExp engine is used instead of the oniguruma WASM
//     build. It keeps the bundle a single JS file with no WASM blob to embed
//     or fetch, which matters because everything ships inside the Go binary.
//   - The language set covers common languages and formats, picked from
//     @shikijs/langs' exports and skipping niche/DSL-specific grammars.
//     Anything else falls back to a plain, unhighlighted code block.
import { createHighlighterCore, type HighlighterCore } from "shiki/core";
import { createJavaScriptRegexEngine } from "shiki/engine/javascript";
import githubLight from "@shikijs/themes/github-light";
import githubDark from "@shikijs/themes/github-dark";
import langAstro from "@shikijs/langs/astro";
import langC from "@shikijs/langs/c";
import langClojure from "@shikijs/langs/clojure";
import langCpp from "@shikijs/langs/cpp";
import langCsharp from "@shikijs/langs/csharp";
import langCss from "@shikijs/langs/css";
import langCsv from "@shikijs/langs/csv";
import langDart from "@shikijs/langs/dart";
import langDiff from "@shikijs/langs/diff";
import langDockerfile from "@shikijs/langs/dockerfile";
import langElixir from "@shikijs/langs/elixir";
import langErlang from "@shikijs/langs/erlang";
import langGitCommit from "@shikijs/langs/git-commit";
import langGitRebase from "@shikijs/langs/git-rebase";
import langGlsl from "@shikijs/langs/glsl";
import langGo from "@shikijs/langs/go";
import langGraphql from "@shikijs/langs/graphql";
import langGroovy from "@shikijs/langs/groovy";
import langHaskell from "@shikijs/langs/haskell";
import langHtml from "@shikijs/langs/html";
import langIni from "@shikijs/langs/ini";
import langJava from "@shikijs/langs/java";
import langJavaScript from "@shikijs/langs/javascript";
import langJson from "@shikijs/langs/json";
import langJson5 from "@shikijs/langs/json5";
import langJsonc from "@shikijs/langs/jsonc";
import langJsx from "@shikijs/langs/jsx";
import langKotlin from "@shikijs/langs/kotlin";
import langLatex from "@shikijs/langs/latex";
import langLess from "@shikijs/langs/less";
import langLua from "@shikijs/langs/lua";
import langMakefile from "@shikijs/langs/makefile";
import langMarkdown from "@shikijs/langs/markdown";
import langNginx from "@shikijs/langs/nginx";
import langObjectiveC from "@shikijs/langs/objective-c";
import langObjectiveCpp from "@shikijs/langs/objective-cpp";
import langPerl from "@shikijs/langs/perl";
import langPhp from "@shikijs/langs/php";
import langProperties from "@shikijs/langs/properties";
import langProtobuf from "@shikijs/langs/protobuf";
import langPython from "@shikijs/langs/python";
import langR from "@shikijs/langs/r";
import langRegex from "@shikijs/langs/regex";
import langRuby from "@shikijs/langs/ruby";
import langRust from "@shikijs/langs/rust";
import langScala from "@shikijs/langs/scala";
import langScss from "@shikijs/langs/scss";
import langShell from "@shikijs/langs/shellscript";
import langSql from "@shikijs/langs/sql";
import langSshConfig from "@shikijs/langs/ssh-config";
import langSvelte from "@shikijs/langs/svelte";
import langSwift from "@shikijs/langs/swift";
import langTerraform from "@shikijs/langs/terraform";
import langToml from "@shikijs/langs/toml";
import langTsx from "@shikijs/langs/tsx";
import langTypeScript from "@shikijs/langs/typescript";
import langVim from "@shikijs/langs/vim";
import langVue from "@shikijs/langs/vue";
import langWasm from "@shikijs/langs/wasm";
import langXml from "@shikijs/langs/xml";
import langYaml from "@shikijs/langs/yaml";

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
    langs: [
      langAstro,
      langC,
      langClojure,
      langCpp,
      langCsharp,
      langCss,
      langCsv,
      langDart,
      langDiff,
      langDockerfile,
      langElixir,
      langErlang,
      langGitCommit,
      langGitRebase,
      langGlsl,
      langGo,
      langGraphql,
      langGroovy,
      langHaskell,
      langHtml,
      langIni,
      langJava,
      langJavaScript,
      langJson,
      langJson5,
      langJsonc,
      langJsx,
      langKotlin,
      langLatex,
      langLess,
      langLua,
      langMakefile,
      langMarkdown,
      langNginx,
      langObjectiveC,
      langObjectiveCpp,
      langPerl,
      langPhp,
      langProperties,
      langProtobuf,
      langPython,
      langR,
      langRegex,
      langRuby,
      langRust,
      langScala,
      langScss,
      langShell,
      langSql,
      langSshConfig,
      langSvelte,
      langSwift,
      langTerraform,
      langToml,
      langTsx,
      langTypeScript,
      langVim,
      langVue,
      langWasm,
      langXml,
      langYaml,
    ],
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
