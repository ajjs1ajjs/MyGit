<template>
  <AppLayout>
    <div class="max-w-4xl mx-auto">
      <div v-if="repo">
        <div class="flex items-center gap-3 mb-4">
          <h1 class="text-xl font-bold">{{ repo.path }}</h1>
          <span class="text-xs px-2 py-0.5 rounded bg-gray-200 dark:bg-slate-700">{{ repo.visibility }}</span>
        </div>
        <div class="flex gap-2 mb-6 flex-wrap">
          <RouterLink :to="`/${repo.path}/-/tree/${repo.default_branch}`" class="px-3 py-1 text-sm border rounded hover:bg-gray-100 dark:hover:bg-slate-800">Files</RouterLink>
          <RouterLink :to="`/${repo.path}/-/commits/${repo.default_branch}`" class="px-3 py-1 text-sm border rounded hover:bg-gray-100 dark:hover:bg-slate-800">Commits</RouterLink>
          <RouterLink :to="`/${repo.path}/-/branches`" class="px-3 py-1 text-sm border rounded hover:bg-gray-100 dark:hover:bg-slate-800">Branches</RouterLink>
          <RouterLink :to="`/${repo.path}/-/tags`" class="px-3 py-1 text-sm border rounded hover:bg-gray-100 dark:hover:bg-slate-800">Tags</RouterLink>
          <RouterLink :to="`/${repo.path}/-/issues`" class="px-3 py-1 text-sm border rounded hover:bg-gray-100 dark:hover:bg-slate-800">Issues</RouterLink>
          <RouterLink :to="`/${repo.path}/-/merge_requests`" class="px-3 py-1 text-sm border rounded hover:bg-gray-100 dark:hover:bg-slate-800">MRs</RouterLink>
          <RouterLink :to="`/${repo.path}/-/wiki`" class="px-3 py-1 text-sm border rounded hover:bg-gray-100 dark:hover:bg-slate-800">Wiki</RouterLink>
          <RouterLink :to="`/${repo.path}/-/settings`" class="px-3 py-1 text-sm border rounded hover:bg-gray-100 dark:hover:bg-slate-800">Settings</RouterLink>
        </div>
        <p v-if="repo.description" class="text-gray-600 dark:text-gray-400 mb-4">{{ repo.description }}</p>

        <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mt-6">
          <div class="border rounded-lg p-3 text-center">
            <div class="text-2xl font-bold">{{ commits.length || '-' }}</div>
            <div class="text-xs text-gray-500 mt-1">Commits</div>
          </div>
          <div class="border rounded-lg p-3 text-center">
            <div class="text-2xl font-bold">{{ branches.length || '-' }}</div>
            <div class="text-xs text-gray-500 mt-1">Branches</div>
          </div>
          <div class="border rounded-lg p-3 text-center">
            <div class="text-2xl font-bold">{{ tags.length || '-' }}</div>
            <div class="text-xs text-gray-500 mt-1">Tags</div>
          </div>
          <div class="border rounded-lg p-3 text-center">
            <div class="text-2xl font-bold">{{ repo.size_kb ? (repo.size_kb / 1024).toFixed(1) + ' MB' : '-' }}</div>
            <div class="text-xs text-gray-500 mt-1">Size</div>
          </div>
        </div>
      </div>
      <p v-else-if="loading" class="text-gray-500">Loading...</p>
      <p v-else class="text-red-500">{{ error || 'Repository not found' }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import AppLayout from "../components/AppLayout.vue";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const { repo, repoId, loading, error } = useRepo(repoUsername, repoName);

const commits = ref<any[]>([]);
const branches = ref<any[]>([]);
const tags = ref<any[]>([]);

watch(repoId, async (id) => {
  if (!id) return;
  try {
    const [c, b, t] = await Promise.all([
      api.get(`/projects/${id}/commits/`),
      api.get(`/projects/${id}/branches/`),
      api.get(`/projects/${id}/tags/`),
    ]);
    commits.value = c?.commits || [];
    branches.value = b?.branches || [];
    tags.value = t?.tags || [];
  } catch {}
});
</script>
