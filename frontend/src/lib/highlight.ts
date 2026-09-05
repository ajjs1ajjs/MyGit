import hljs from "highlight.js/lib/core";
import javascript from "highlight.js/lib/languages/javascript";
import typescript from "highlight.js/lib/languages/typescript";
import python from "highlight.js/lib/languages/python";
import json from "highlight.js/lib/languages/json";
import yaml from "highlight.js/lib/languages/yaml";
import ini from "highlight.js/lib/languages/ini";
import bash from "highlight.js/lib/languages/bash";
import ruby from "highlight.js/lib/languages/ruby";
import go from "highlight.js/lib/languages/go";
import rust from "highlight.js/lib/languages/rust";
import java from "highlight.js/lib/languages/java";
import kotlin from "highlight.js/lib/languages/kotlin";
import c from "highlight.js/lib/languages/c";
import cpp from "highlight.js/lib/languages/cpp";
import csharp from "highlight.js/lib/languages/csharp";
import php from "highlight.js/lib/languages/php";
import xml from "highlight.js/lib/languages/xml";
import css from "highlight.js/lib/languages/css";
import scss from "highlight.js/lib/languages/scss";
import sql from "highlight.js/lib/languages/sql";
import markdown from "highlight.js/lib/languages/markdown";
import dockerfile from "highlight.js/lib/languages/dockerfile";

// Register only the languages the UI can actually pick (see the langMap in
// RepoBlobPage). Importing the full "highlight.js" package bundles ~200
// languages (~1MB); core + these ~22 keeps the bundle to a few dozen KB.
// Order matters: sublanguages (cpp->c, scss->css) must be registered first.
export function registerLanguages() {
  const languages: [string, any][] = [
    ["javascript", javascript],
    ["typescript", typescript],
    ["python", python],
    ["json", json],
    ["yaml", yaml],
    ["toml", ini], // no TOML grammar ships with highlight.js; INI is close enough
    ["bash", bash],
    ["ruby", ruby],
    ["go", go],
    ["rust", rust],
    ["java", java],
    ["kotlin", kotlin],
    ["c", c],
    ["cpp", cpp],
    ["csharp", csharp],
    ["php", php],
    ["xml", xml],
    ["css", css],
    ["scss", scss],
    ["sql", sql],
    ["markdown", markdown],
    ["dockerfile", dockerfile],
  ];
  for (const [name, lang] of languages) {
    if (!hljs.getLanguage(name)) {
      hljs.registerLanguage(name, lang);
    }
  }
}

export { hljs };

