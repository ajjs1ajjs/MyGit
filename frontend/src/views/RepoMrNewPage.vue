<template>
  <div class="max-w-2xl mx-auto">
      <div class="mb-4">
        <RouterLink :to="`/${repoUsername}/${repoName}/-/merge_requests`" class="text-sm text-blue-600 hover:underline">&larr; Merge Requests</RouterLink>
      </div>
      <h1 class="text-xl font-bold mb-6">New Merge Request</h1>
      <p v-if="error" class="text-red-500 text-sm mb-4">{{ error }}</p>
      <form @submit.prevent="create" class="flex flex-col gap-4">
        <input v-model="title" placeholder="Title" required class="px-3 py-2 border rounded text-sm" />
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="text-xs text-gray-500">Source branch</label>
            <select v-model="sourceBranch" class="w-full px-3 py-2 border rounded text-sm bg-white dark:bg-slate-800">
              <option v-for="b in branches" :key="b.name" :value="b.name">{{ b.name }}</option>
            </select>
          </div>
          <div>
            <label class="text-xs text-gray-500">Target branch</label>
            <select v-model="targetBranch" class="w-full px-3 py-2 border rounded text-sm bg-white dark:bg-slate-800">
              <option v-for="b in branches" :key="b.name" :value="b.name">{{ b.name }}</option>
            </select>
          </div>
        </div>
        <textarea v-model="description" placeholder="Description (Markdown)" rows="5" class="px-3 py-2 border rounded text-sm"></textarea>
        <button type="submit" :disabled="loading" class="self-start px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50">
          {{ loading ? 'Creating...' : 'Create merge request' }}
        </button>
      </form>
    </div>
  </template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const router = useRouter();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const { repoId } = useRepo(repoUsername, repoName);

const title = ref("");
const description = ref("");
const sourceBranch = ref("");
const targetBranch = ref("");
const branches = ref<any[]>([]);
const loading = ref(false);
const error = ref("");

watch(repoId, async (id) => {
  if (!id) return;
  try {
    const data = await api.get(`/projects/${id}/branches/`);
    branches.value = data?.branches || [];
    if (branches.value.length > 1) {
      sourceBranch.value = branches.value[1].name;
      targetBranch.value = branches.value[0].name;
    } else if (branches.value.length === 1) {
      sourceBranch.value = branches.value[0].name;
      targetBranch.value = branches.value[0].name;
    }
  } catch {}
});

async function create() {
  if (!repoId.value) return;
  loading.value = true;
  error.value = "";
  try {
    const mr = await api.post(`/projects/${repoId.value}/merge_requests/`, {
      title: title.value,
      description: description.value,
      source_branch: sourceBranch.value,
      target_branch: targetBranch.value,
    });
    router.push(`/${repoUsername}/${repoName}/-/merge_requests/${mr.number}`);
  } catch (e: any) {
    error.value = e.message;
  }
  loading.value = false;
}
</script>
