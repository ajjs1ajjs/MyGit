import { marked } from "marked";
import DOMPurify from "dompurify";

export function renderMarkdown(text: string | null | undefined): string {
  if (!text) return "";
  try {
    const raw = marked.parse(text) as string;
    return DOMPurify.sanitize(raw, {
      USE_PROFILES: { html: true },
      FORBID_TAGS: ["style", "form", "input", "button", "textarea", "select", "option", "iframe", "object", "embed", "link", "meta", "base", "script", "noscript"],
      // Belt-and-suspenders over DOMPurify's default event-handler stripping:
      // every on* attribute must go, not just the four most common ones.
      FORBID_ATTR: ["onerror", "onload", "onclick", "onmouseover", "onmouseout", "onmouseenter", "onmouseleave", "onfocus", "onblur", "onsubmit", "onchange", "onkeydown", "onkeyup", "onkeypress", "ondblclick", "oncontextmenu", "onanimationstart", "ontoggle", "formaction", "xlink:href"],
    });
  } catch {
    return "";
  }
}
