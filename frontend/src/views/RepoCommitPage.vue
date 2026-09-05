<template>
  <div class="max-w-4xl mx-auto" v-if="commit">
      <div class="mb-4">
        <RouterLink :to="`/${repoUsername}/${repoName}/-/commits/${commitSha}`" class="text-sm text-blue-600 hover:underline">&larr; Commits</RouterLink>
      </div>
      <h1 class="text-lg font-semibold mb-1">{{ commit.message }}</h1>
      <p class="text-sm text-gray-500 mb-4">
        {{ commit.short_sha }} &middot; {{ commit.author?.name }} &middot; {{ new Date(commit.committed_at).toLocaleString() }}
      </p>
      <div v-if="parents.length" class="mb-4 text-sm text-gray-500">
        Parents: {{ parents.join(", ") }}
      </div>

      <div class="mb-6">
        <h3 class="font-semibold mb-2">Files changed</h3>
        <div v-if="diffs.length" class="border rounded divide-y">
          <FileDiff v-for="(d, i) in diffs" :key="i" :diff="d" />
        </div>
        <p v-else class="text-sm text-gray-500">No changes.</p>
      </div>
    </div>
    <div v-else class="max-w-4xl mx-auto">
      <p v-if="loading" class="text-gray-500">Loading...</p>
      <p v-else class="text-red-500">{{ error }}</p>
    </div>
  </template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import FileDiff from "../components/FileDiff.vue";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const commitSha = route.params.sha as string;
const { repoId, loading, error } = useRepo(repoUsername, repoName);

const commit = ref<any>(null);
const diffs = ref<any[]>([]);
const parents = ref<string[]>([]);

watch(repoId, async (id) => {
  if (!id) return;
  try {
    commit.value = await api.get(`/projects/${id}/commits/${commitSha}/`);
    parents.value = commit.value?.parents || [];
    const diffData = await api.get(`/projects/${id}/commits/${commitSha}/diff/`);
    diffs.value = diffData?.diffs || [];
  } catch (e: any) {
    error.value = e.message;
  }
});
</script>
