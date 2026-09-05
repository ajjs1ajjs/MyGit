<template>
  <details class="group">
    <summary class="px-4 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-800 font-mono">
      <span :class="diff.type === 'D' ? 'text-red-600' : diff.type === 'A' ? 'text-green-600' : ''">{{ diff.new_path || diff.old_path }}</span>
      <span class="text-xs text-gray-400 ml-2">{{ diff.type }}</span>
    </summary>
    <pre class="px-4 py-2 bg-gray-50 dark:bg-slate-900 text-xs overflow-x-auto border-t"><code v-html="highlightDiff(diff.diff)"></code></pre>
  </details>
</template>

<script setup lang="ts">
defineProps<{ diff: { type: string; old_path: string; new_path: string; diff: string } }>();

function highlightDiff(text: string) {
  if (!text) return "";
  return text
    .split("\n")
    .map((line) => {
      if (line.startsWith("+")) return `<span class="text-green-600">${escapeHtml(line)}</span>`;
      if (line.startsWith("-")) return `<span class="text-red-600">${escapeHtml(line)}</span>`;
      if (line.startsWith("@@")) return `<span class="text-blue-600">${escapeHtml(line)}</span>`;
      return escapeHtml(line);
    })
    .join("\n");
}

function escapeHtml(s: string) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}
</script>
