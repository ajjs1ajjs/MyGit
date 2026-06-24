<template>
  <div class="max-w-4xl mx-auto">
      <h2 class="font-semibold mb-3">Commits: {{ branchRef }}</h2>
      <div v-if="commits.length" class="border rounded divide-y">
        <RouterLink
          v-for="c in commits"
          :key="c.sha"
          :to="`/${repoUsername}/${repoName}/-/commit/${c.sha}`"
          class="px-4 py-3 flex items-start gap-3 hover:bg-gray-50 dark:hover:bg-slate-800"
        >
          <div class="flex-1">
            <div class="text-sm font-medium">{{ c.message }}</div>
            <div class="text-xs text-gray-500 mt-1">
              <span class="font-mono">{{ c.short_sha }}</span>
              &middot; {{ c.author?.name }}
              &middot; {{ new Date(c.committed_at).toLocaleString() }}
            </div>
          </div>
        </RouterLink>
      </div>
      <p v-else-if="!loading" class="text-sm text-gray-500">No commits yet.</p>
      <p v-if="loading" class="text-sm text-gray-500">Loading...</p>
    </div>
  </template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const fullRef = (route.params.ref as string) || "main";
const branchRef = fullRef.split("/")[0];
const { repoId, loading } = useRepo(repoUsername, repoName);
const commits = ref<any[]>([]);

watch(repoId, async (id) => {
  if (!id) return;
  try {
    const data = await api.get(`/projects/${id}/commits/?ref=${branchRef}`);
    commits.value = data?.commits || [];
  } catch {}
});
</script>
