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

    <!-- Blame Card -->
    <div class="bg-white dark:bg-slate-900 border rounded-lg overflow-hidden">
      <div class="px-4 py-2 bg-gray-100 dark:bg-slate-800 border-b text-xs font-mono text-gray-500 flex justify-between items-center">
        <span>{{ filePath }} ({{ blameLines.length }} lines)</span>
        <div class="flex items-center gap-2">
          <RouterLink :to="`/${repoUsername}/${repoName}/-/blob/${refParam}?path=${encodeURIComponent(filePath)}`" class="btn btn-ghost text-xs px-2 py-1 min-h-0 hover:bg-blue-100 dark:hover:bg-blue-900/30" title="View file">
            <svg class="w-3.5 h-3.5 mr-1" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg>
            View
          </RouterLink>
          <RouterLink :to="getRawLink()" class="btn btn-ghost text-xs px-2 py-1 min-h-0 hover:bg-blue-100 dark:hover:bg-blue-900/30" target="_blank" rel="noopener" title="Raw">
            <svg class="w-3.5 h-3.5 mr-1" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>
            Raw
          </RouterLink>
        </div>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-sm font-mono">
          <tbody>
            <tr v-for="(line, i) in blameLines" :key="i" class="hover:bg-gray-50 dark:hover:bg-slate-800">
              <td class="pl-4 pr-3 py-0.5 text-right text-xs text-gray-400 select-none border-r w-12">
                {{ i + 1 }}
              </td>
              <td class="pl-4 pr-3 py-0.5 text-right text-xs text-gray-400 select-none border-r w-10" title="{{ line.sha }}">
                {{ line.short_sha }}
              </td>
              <td class="pl-3 pr-3 py-0.5 text-left text-xs text-gray-500 select-none border-r w-32" title="{{ line.author }} ({{ line.author_email }})">
                {{ line.author }}
              </td>
              <td class="pl-3 pr-3 py-0.5 text-left text-xs text-gray-500 select-none border-r w-24">
                {{ formatDate(line.committed_at) }}
              </td>
              <td class="pl-4 py-0.5 whitespace-pre-wrap break-words"><code>{{ line.line }}</code></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <p v-if="loading" class="text-gray-500 mt-4 text-sm">Loading...</p>
    <p v-if="error" class="text-red-500 mt-4 text-sm">{{ error }}</p>
    <p v-if="!loading && !error && blameLines.length === 0" class="text-gray-500 mt-4 text-sm">No blame data available.</p>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const refParam = ((route.params.ref as string) || "main");
const filePath = (route.query.path as string) || "";
const { repoId, loading, error } = useRepo(repoUsername, repoName);

const blameLines = ref<any[]>([]);

const filePathParts = computed(() => {
  if (!filePath) return [];
  return filePath.split("/");
});

function getBreadcrumbLink(index: number) {
  const parts = filePathParts.value.slice(0, index + 1);
  return `/${repoUsername}/${repoName}/-/tree/${refParam}/${parts.join("/")}`;
}

function getRawLink() {
  const ref = refParam || "main";
  return `/api/v1/projects/${repoId.value}/raw/?ref=${encodeURIComponent(ref)}&path=${encodeURIComponent(filePath)}`;
}

function formatDate(dateStr: string) {
  if (!dateStr) return "";
  return new Date(dateStr).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

watch(repoId, async (id) => {
  if (!id) return;
  try {
    const url = `/projects/${id}/blame/?ref=${encodeURIComponent(refParam)}&path=${encodeURIComponent(filePath)}`;
    const data = await api.get(url);
    blameLines.value = data?.lines || [];
  } catch (e: any) {
    error.value = e.message;
  }
});
</script>