<template>
  <div class="max-w-2xl mx-auto">
      <div class="mb-4">
        <RouterLink :to="`/${repoUsername}/${repoName}/-/issues`" class="text-sm text-blue-600 hover:underline">&larr; Issues</RouterLink>
      </div>
      <h1 class="text-xl font-bold mb-6">New Issue</h1>
      <p v-if="error" class="text-red-500 text-sm mb-4">{{ error }}</p>
      <form @submit.prevent="create" class="flex flex-col gap-4">
        <input v-model="title" placeholder="Title" required class="px-3 py-2 border rounded text-sm" />
        <textarea v-model="description" placeholder="Description (Markdown)" rows="6" class="px-3 py-2 border rounded text-sm"></textarea>
        <button type="submit" :disabled="loading" class="self-start px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50">
          {{ loading ? 'Creating...' : 'Create issue' }}
        </button>
      </form>
    </div>
  </template>

<script setup lang="ts">
import { ref } from "vue";
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
const loading = ref(false);
const error = ref("");

async function create() {
  if (!repoId.value) return;
  loading.value = true;
  error.value = "";
  try {
    const issue = await api.post(`/projects/${repoId.value}/issues/`, {
      title: title.value,
      description: description.value,
    });
    router.push(`/${repoUsername}/${repoName}/-/issues/${issue.number}`);
  } catch (e: any) {
    error.value = e.message;
  }
  loading.value = false;
}
</script>
