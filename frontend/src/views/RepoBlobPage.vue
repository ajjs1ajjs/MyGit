<template>
  <div class="max-w-5xl mx-auto">
    <!-- Breadcrumbs -->
    <div class="mb-3 flex items-center gap-2 text-sm">
      <RouterLink :to="`/${repoUsername}/${repoName}`" class="text-blue-600 hover:underline">{{ repoUsername }}/{{ repoName }}</RouterLink>
      <template v-if="filePathParts.length">
        <span class="text-gray-400">/</span>
        <template v-for="(part, idx) in filePathParts" :key="idx">
          <RouterLink v-if="idx < filePathParts.length - 1" :to="getBreadcrumbLink(idx)" class="text-blue-600 hover:underline">{{ part }}</RouterLink>
          <span v-else class="text-gray-600 dark:text-gray-300 font-semibold">{{ part }}</span>
          <span v-if="idx < filePathParts.length - 1" class="text-gray-400">/</span>
        </template>
      </template>
    </div>

    <!-- Blob Card -->
    <div class="bg-white dark:bg-slate-900 border rounded-lg overflow-hidden">
      <div class="px-4 py-2 bg-gray-100 dark:bg-slate-800 border-b text-xs font-mono text-gray-500 flex justify-between items-center">
        <span>{{ filePath }} ({{ lines }} lines)</span>
        <div class="flex items-center gap-4">
          <!-- View Mode Switcher for Markdown -->
          <div v-if="isMarkdown" class="flex border rounded overflow-hidden">
            <button 
              @click="viewMode = 'preview'" 
              :class="viewMode === 'preview' ? 'bg-blue-600 text-white border-blue-600' : 'bg-transparent text-gray-500 hover:bg-gray-200 dark:hover:bg-slate-700'"
              class="px-2.5 py-1 text-[10px] border-0 cursor-pointer font-sans font-medium transition-colors">
              Preview
            </button>
            <button 
              @click="viewMode = 'source'" 
              :class="viewMode === 'source' ? 'bg-blue-600 text-white border-blue-600' : 'bg-transparent text-gray-500 hover:bg-gray-200 dark:hover:bg-slate-700'"
              class="px-2.5 py-1 text-[10px] border-0 cursor-pointer font-sans font-medium transition-colors">
              Source
            </button>
          </div>
          <span>{{ content?.length || 0 }} bytes</span>
          <RouterLink :to="getRawLink()" class="btn btn-ghost text-xs px-2 py-1 min-h-0 hover:bg-blue-100 dark:hover:bg-blue-900/30" target="_blank" rel="noopener">
            <svg class="w-3.5 h-3.5 mr-1" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>
            Raw
          </RouterLink>
          <RouterLink :to="getBlameLink()" class="btn btn-ghost text-xs px-2 py-1 min-h-0 hover:bg-orange-100 dark:hover:bg-orange-900/30">
            <svg class="w-3.5 h-3.5 mr-1" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle></svg>
            Blame
          </RouterLink>
        </div>
      </div>
      
      <div class="overflow-x-auto">
        <!-- Rendered Markdown View -->
        <div v-if="isMarkdown && viewMode === 'preview'" class="p-6 markdown-body" v-html="renderedMarkdown"></div>

        <!-- Code Source View -->
        <table v-else-if="content" class="w-full text-sm font-mono">
          <tbody>
            <tr v-for="(line, i) in contentLines" :key="i" class="hover:bg-gray-50 dark:hover:bg-slate-800">
              <td class="pl-4 pr-3 py-0.5 text-right text-xs text-gray-400 select-none border-r w-12">{{ i + 1 }}</td>
              <td class="pl-4 py-0.5 whitespace-pre-wrap break-words">
                <code :class="['hljs', 'language-' + language]" v-html="highlightedLine(line, i)"></code>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <p v-if="loading" class="text-gray-500 mt-4 text-sm">Loading...</p>
    <p v-if="error" class="text-red-500 mt-4 text-sm">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import { useRepo } from "../composables/useRepo";
import { renderMarkdown } from "../utils/markdown";
import { hljs, registerLanguages } from "../lib/highlight";
import "highlight.js/styles/github-dark.css";

registerLanguages();

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const refParam = ((route.params.ref as string) || "main");
const filePath = (route.query.path as string) || "";
const { repoId, loading, error } = useRepo(repoUsername, repoName);

const content = ref("");
const lines = ref(0);
const viewMode = ref("preview"); // "preview" | "source"

const contentLines = computed(() => content.value.split("\n"));

const filePathParts = computed(() => {
  if (!filePath) return [];
  return filePath.split("/");
});

const isMarkdown = computed(() => {
  return filePath.toLowerCase().endsWith(".md");
});

const language = computed(() => {
  const ext = filePath.split(".").pop()?.toLowerCase();
  const langMap: Record<string, string> = {
    py: "python",
    js: "javascript",
    ts: "typescript",
    tsx: "typescript",
    jsx: "javascript",
    json: "json",
    yml: "yaml",
    yaml: "yaml",
    toml: "toml",
    sh: "bash",
    bash: "bash",
    zsh: "bash",
    dockerfile: "dockerfile",
    docker: "dockerfile",
    rb: "ruby",
    go: "go",
    rs: "rust",
    java: "java",
    kt: "kotlin",
    c: "c",
    h: "c",
    cpp: "cpp",
    cc: "cpp",
    hpp: "cpp",
    cs: "csharp",
    php: "php",
    html: "xml",
    xml: "xml",
    css: "css",
    scss: "scss",
    sql: "sql",
    md: "markdown",
  };
  return langMap[ext || ""] || "plaintext";
});

const renderedMarkdown = computed(() => renderMarkdown(content.value));

function getBreadcrumbLink(index: number) {
  const parts = filePathParts.value.slice(0, index + 1);
  return `/${repoUsername}/${repoName}/-/tree/${refParam}/${parts.join("/")}`;
}

function getRawLink() {
  const ref = refParam || "main";
  return `/${repoUsername}/${repoName}/-/raw/${ref}/${encodeURIComponent(filePath)}`;
}

function getBlameLink() {
  const ref = refParam || "main";
  return `/${repoUsername}/${repoName}/-/blame/${ref}?path=${encodeURIComponent(filePath)}`;
}

function highlightedLine(line: string, index: number): string {
  if (!content.value) return "";
  try {
    return hljs.highlight(line, { language: language.value }).value;
  } catch (e) {
    return line;
  }
}

watch(repoId, async (id) => {
  if (!id) return;
  try {
    const sha = filePath ? "0" : refParam;
    let url = `/projects/${id}/blobs/${sha}/?ref=${refParam}`;
    if (filePath) url += `&path=${encodeURIComponent(filePath)}`;
    const data = await api.get(url);
    content.value = data?.encoding === "base64" ? atob(data.content || "") : (data?.content || "");
    lines.value = content.value.split("\n").length;
  } catch (e: any) {
    error.value = e.message;
  }
});

watch([renderedMarkdown, viewMode], () => {
  if (viewMode.value === "preview" && isMarkdown.value) {
    nextTick(() => {
      document.querySelectorAll(".markdown-body pre code").forEach((el) => {
        hljs.highlightElement(el as HTMLElement);
      });
    });
  }
}, { immediate: true });
</script>

