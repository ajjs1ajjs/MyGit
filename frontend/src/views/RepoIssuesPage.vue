<template>
  <div class="max-w-4xl mx-auto">
      <div class="flex items-center justify-between mb-4">
        <h2 class="font-semibold">Issues</h2>
        <RouterLink :to="`/${repoUsername}/${repoName}/-/issues/new`" class="px-3 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-700">New Issue</RouterLink>
      </div>
      <div v-if="issues.length" class="border rounded divide-y">
        <RouterLink
          v-for="i in issues"
          :key="i.number"
          :to="`/${repoUsername}/${repoName}/-/issues/${i.number}`"
          class="px-4 py-3 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-slate-800"
        >
          <span :class="i.state === 'open' ? 'text-green-600' : 'text-red-600'" class="text-lg leading-none">&bull;</span>
          <div class="flex-1">
            <span :class="i.state === 'closed' ? 'line-through text-gray-400' : ''" class="text-sm">{{ i.title }}</span>
            <div class="text-xs text-gray-500 mt-0.5">#{{ i.number }} opened by {{ i.author_username }} &middot; {{ new Date(i.created_at).toLocaleDateString() }}</div>
          </div>
          <div v-if="i.labels?.length" class="flex gap-1">
            <span v-for="l in i.labels" :key="l.id" :style="{ background: l.color + '20', color: l.color, borderColor: l.color }" class="px-2 py-0.5 text-xs rounded-full border">
              {{ l.name }}
            </span>
          </div>
        </RouterLink>
      </div>
      <p v-else-if="!loading" class="text-sm text-gray-500">No issues yet.</p>
      <p v-if="loading" class="text-sm text-gray-500">Loading...</p>
      <p v-if="error" class="text-sm text-red-500">{{ error }}</p>
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
const { repoId, loading, error } = useRepo(repoUsername, repoName);
const issues = ref<any[]>([]);

watch(repoId, async (id) => {
  if (!id) return;
  try {
    issues.value = (await api.get(`/projects/${id}/issues/`)) || [];
  } catch {}
});
</script>
