<template>
  <div class="max-w-4xl mx-auto">
      <div class="mb-3 text-sm text-gray-500">
        <RouterLink :to="`/${repoUsername}/${repoName}`" class="text-blue-600 hover:underline">{{ repoUsername }}/{{ repoName }}</RouterLink>
        <span v-if="pathParts.length"> / <template v-for="(part, idx) in pathParts" :key="idx">
          <RouterLink v-if="idx < pathParts.length - 1" :to="treeLink(idx + 1)" class="text-blue-600 hover:underline">{{ part }}</RouterLink>
          <span v-else>{{ part }}</span>
          <span v-if="idx < pathParts.length - 1"> / </span>
        </template></span>
        <span class="ml-2">(@{{ branchRef }})</span>
      </div>
      <div v-if="entries.length" class="border rounded divide-y">
        <div v-for="e in entries" :key="e.sha" class="px-4 py-2 flex items-center gap-2 text-sm hover:bg-gray-50 dark:hover:bg-slate-800">
          <span class="text-base">{{ e.type === "tree" ? "📁" : "📄" }}</span>
          <RouterLink
            v-if="e.type === 'tree'"
            :to="`/${repoUsername}/${repoName}/-/tree/${branchRef}/${e.path}`"
            class="text-blue-600 hover:underline flex-1"
          >{{ e.name }}</RouterLink>
          <RouterLink
            v-else
            :to="`/${repoUsername}/${repoName}/-/blob/${branchRef}?path=${encodeURIComponent(e.path)}`"
            class="flex-1 hover:text-blue-600"
          >{{ e.name }}</RouterLink>
        </div>
      </div>
      <p v-else-if="!loading" class="text-gray-500 text-sm">Empty directory.</p>
      <p v-if="loading" class="text-gray-500 text-sm">Loading...</p>
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
const fullRef = (route.params.ref as string) || "main";
const { repoId, loading } = useRepo(repoUsername, repoName);

const parts = fullRef.split("/");
const branchRef = parts[0];
const dirPath = parts.slice(1).join("/");

const pathParts = computed(() => parts.slice(1));
const entries = ref<any[]>([]);

function treeLink(count: number) {
  const p = parts.slice(0, 1 + count).join("/");
  return `/${repoUsername}/${repoName}/-/tree/${p}`;
}

watch(repoId, async (id) => {
  if (!id) return;
  try {
    let url = `/projects/${id}/tree/?ref=${branchRef}`;
    if (dirPath) url += `&path=${encodeURIComponent(dirPath)}`;
    const data = await api.get(url);
    entries.value = data?.entries || [];
  } catch {}
});
</script>
