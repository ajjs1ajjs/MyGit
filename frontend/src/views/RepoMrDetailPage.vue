<template>
  <AppLayout>
    <div class="max-w-4xl mx-auto" v-if="mr">
      <div class="mb-4">
        <RouterLink :to="`/${repoUsername}/${repoName}/-/merge_requests`" class="text-sm text-blue-600 hover:underline">&larr; Merge Requests</RouterLink>
      </div>
      <div class="mb-4">
        <h1 class="text-xl font-bold">{{ mr.title }}</h1>
        <p class="text-sm text-gray-500 mt-1">
          !{{ mr.number }} &middot; {{ mr.author_username }} wants to merge {{ mr.source_branch }} into {{ mr.target_branch }}
        </p>
        <span :class="stateClass" class="inline-block mt-2 px-2 py-0.5 text-xs rounded-full font-medium">{{ mr.state }}</span>
      </div>

      <div class="bg-white dark:bg-slate-800 border rounded-lg p-4 mb-6" v-if="mr.description">
        <div class="prose dark:prose-invert max-w-none text-sm" v-html="renderMd(mr.description)"></div>
      </div>

      <div v-if="mr.state === 'open' || mr.state === 'draft'" class="flex gap-2 mb-6">
        <button @click="doMerge('merge-commit')" class="px-4 py-1.5 bg-blue-600 text-white text-sm rounded hover:bg-blue-700">Merge</button>
        <select v-model="mergeMethod" class="px-3 py-1.5 border rounded text-sm bg-white dark:bg-slate-800">
          <option value="merge-commit">Merge commit</option>
          <option value="fast-forward">Fast-forward</option>
        </select>
        <p v-if="mergeError" class="text-red-500 text-sm">{{ mergeError }}</p>
      </div>

      <div class="mb-6">
        <h3 class="font-semibold mb-2">Changes</h3>
        <div v-if="diffs.length" class="border rounded divide-y">
          <FileDiff v-for="(d, i) in diffs" :key="i" :diff="d" />
        </div>
        <p v-else class="text-sm text-gray-500">No changes.</p>
      </div>

      <h3 class="font-semibold mb-3">Comments ({{ comments.length }})</h3>
      <div class="border rounded-lg divide-y mb-4">
        <div v-for="c in comments" :key="c.id" class="p-4">
          <div class="flex items-center gap-2 mb-1">
            <span class="text-sm font-medium">{{ c.author_username }}</span>
            <span class="text-xs text-gray-400">{{ new Date(c.created_at).toLocaleDateString() }}</span>
          </div>
          <div class="prose dark:prose-invert max-w-none text-sm" v-html="renderMd(c.body)"></div>
        </div>
      </div>

      <form @submit.prevent="addComment" class="flex flex-col gap-2">
        <textarea v-model="newComment" placeholder="Write a comment..." rows="3" class="px-3 py-2 border rounded text-sm" required></textarea>
        <button type="submit" class="self-start px-4 py-1.5 bg-blue-600 text-white text-sm rounded hover:bg-blue-700">Comment</button>
      </form>
    </div>
    <div v-else class="max-w-4xl mx-auto">
      <p v-if="loading" class="text-gray-500">Loading...</p>
      <p v-else class="text-red-500">{{ error }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { marked } from "marked";
import AppLayout from "../components/AppLayout.vue";
import FileDiff from "../components/FileDiff.vue";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const router = useRouter();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const mrNum = Number(route.params.num);
const { repoId, loading, error } = useRepo(repoUsername, repoName);

const mr = ref<any>(null);
const comments = ref<any[]>([]);
const diffs = ref<any[]>([]);
const newComment = ref("");
const mergeMethod = ref("merge-commit");
const mergeError = ref("");

const stateClass = computed(() => {
  const state = mr.value?.state || "";
  const map: Record<string, string> = {
    open: "bg-green-100 text-green-800",
    draft: "bg-gray-100 text-gray-800",
    merged: "bg-purple-100 text-purple-800",
    closed: "bg-red-100 text-red-800",
  };
  return map[state] || "";
});

function renderMd(text: string) {
  if (!text) return "";
  return marked.parse(text);
}

async function load() {
  if (!repoId.value) return;
  try {
    mr.value = await api.get(`/projects/${repoId.value}/merge_requests/${mrNum}/`);
    const commentData = await api.get(`/projects/${repoId.value}/merge_requests/${mrNum}/comments/`);
    comments.value = commentData?.results || commentData || [];
    const diffData = await api.get(`/projects/${repoId.value}/merge_requests/${mrNum}/diff/`);
    diffs.value = diffData?.diffs || [];
  } catch (e: any) {
    error.value = e.message;
  }
}

async function doMerge(method: string) {
  if (!repoId.value) return;
  mergeError.value = "";
  try {
    await api.post(`/projects/${repoId.value}/merge_requests/${mrNum}/merge/`, { method });
    mr.value.state = "merged";
  } catch (e: any) {
    mergeError.value = e.message;
  }
}

async function addComment() {
  if (!repoId.value) return;
  try {
    const c = await api.post(`/projects/${repoId.value}/merge_requests/${mrNum}/comments/`, { body: newComment.value });
    comments.value.push(c);
    newComment.value = "";
  } catch (e: any) {}
}

watch(repoId, load);
</script>
