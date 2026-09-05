import { marked } from "marked";
import DOMPurify from "dompurify";

export function renderMarkdown(text: string | null | undefined): string {
  if (!text) return "";
  try {
    const raw = marked.parse(text) as string;
    return DOMPurify.sanitize(raw, {
      USE_PROFILES: { html: true },
      FORBID_TAGS: ["style", "form", "input", "button", "iframe", "object", "embed"],
      FORBID_ATTR: ["onerror", "onload", "onclick", "onmouseover"],
    });
  } catch {
    return "";
  }
}
