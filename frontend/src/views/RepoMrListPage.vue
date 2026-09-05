<template>
  <div class="max-w-4xl mx-auto">
      <div class="flex items-center justify-between mb-4">
        <h2 class="font-semibold">Merge Requests</h2>
        <RouterLink :to="`/${repoUsername}/${repoName}/-/merge_requests/new`" class="px-3 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-700">New MR</RouterLink>
      </div>
      <div v-if="mrs.length" class="border rounded divide-y">
        <RouterLink
          v-for="m in mrs"
          :key="m.number"
          :to="`/${repoUsername}/${repoName}/-/merge_requests/${m.number}`"
          class="px-4 py-3 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-slate-800"
        >
          <span :class="stateIconClass(m.state)" class="text-lg leading-none">&bull;</span>
          <div class="flex-1">
            <span class="text-sm">{{ m.title }}</span>
            <div class="text-xs text-gray-500 mt-0.5">
              !{{ m.number }} &middot; {{ m.source_branch }} → {{ m.target_branch }}
              &middot; {{ m.author_username }} &middot; {{ new Date(m.created_at).toLocaleDateString() }}
            </div>
          </div>
          <span :class="stateBadgeClass(m.state)" class="px-2 py-0.5 text-xs rounded-full font-medium">{{ m.state }}</span>
        </RouterLink>
      </div>
      <p v-else-if="!loading" class="text-sm text-gray-500">No merge requests yet.</p>
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
const mrs = ref<any[]>([]);

function stateIconClass(state: string) {
  const map: Record<string, string> = {
    open: "text-green-600", merged: "text-purple-600",
    closed: "text-red-600", draft: "text-gray-400",
  };
  return map[state] || "";
}

function stateBadgeClass(state: string) {
  const map: Record<string, string> = {
    open: "bg-green-100 text-green-800", merged: "bg-purple-100 text-purple-800",
    closed: "bg-red-100 text-red-800", draft: "bg-gray-100 text-gray-700",
  };
  return map[state] || "";
}

watch(repoId, async (id) => {
  if (!id) return;
  try {
    mrs.value = (await api.get(`/projects/${id}/merge_requests/`)) || [];
  } catch {}
});
</script>
