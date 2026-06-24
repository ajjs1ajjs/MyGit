<template>
  <div class="max-w-4xl mx-auto" v-if="issue">
      <div class="mb-4">
        <RouterLink :to="`/${repoUsername}/${repoName}/-/issues`" class="text-sm text-blue-600 hover:underline">&larr; Issues</RouterLink>
      </div>
      <div class="mb-6">
        <div class="flex items-center gap-3 mb-2">
          <h1 class="text-xl font-bold">{{ issue.title }}</h1>
          <span :class="issue.state === 'open' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'" class="px-2 py-0.5 text-xs rounded-full font-medium">
            {{ issue.state }}
          </span>
        </div>
        <p class="text-sm text-gray-500">
          #{{ issue.number }} opened by {{ issue.author_username }}
          &middot; {{ new Date(issue.created_at).toLocaleDateString() }}
        </p>
      </div>

      <div class="bg-white dark:bg-slate-800 border rounded-lg p-4 mb-6">
        <div class="prose dark:prose-invert max-w-none text-sm" v-html="renderMd(issue.description)"></div>
      </div>

      <div class="flex gap-2 mb-6">
        <button
          @click="toggleState"
          :class="issue.state === 'open' ? 'bg-red-500 hover:bg-red-600' : 'bg-green-500 hover:bg-green-600'"
          class="px-4 py-1.5 text-white text-sm rounded"
        >
          {{ issue.state === 'open' ? 'Close issue' : 'Reopen issue' }}
        </button>
      </div>

      <h3 class="font-semibold mb-3">Comments ({{ comments.length }})</h3>
      <div class="border rounded-lg divide-y mb-4">
        <div v-for="c in comments" :key="c.id" class="p-4">
          <div class="flex items-center gap-2 mb-2">
            <span class="text-sm font-medium">{{ c.author_username }}</span>
            <span class="text-xs text-gray-400">{{ new Date(c.created_at).toLocaleDateString() }}</span>
          </div>
          <div class="prose dark:prose-invert max-w-none text-sm" v-html="renderMd(c.body)"></div>
        </div>
        <div v-if="!comments.length" class="p-4 text-sm text-gray-500">No comments yet.</div>
      </div>

      <form @submit.prevent="addComment" class="flex flex-col gap-2">
        <textarea v-model="newComment" placeholder="Write a comment..." rows="3" class="px-3 py-2 border rounded text-sm" required></textarea>
        <p v-if="commentError" class="text-red-500 text-xs">{{ commentError }}</p>
        <button type="submit" :disabled="commentLoading" class="self-start px-4 py-1.5 bg-blue-600 text-white text-sm rounded hover:bg-blue-700 disabled:opacity-50">
          {{ commentLoading ? 'Posting...' : 'Comment' }}
        </button>
      </form>
    </div>
    <div v-else class="max-w-4xl mx-auto">
      <p v-if="loading" class="text-gray-500">Loading...</p>
      <p v-else class="text-red-500">{{ error }}</p>
    </div>
  </template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import { marked } from "marked";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const issueNum = Number(route.params.num);

const { repoId, loading, error } = useRepo(repoUsername, repoName);
const issue = ref<any>(null);
const comments = ref<any[]>([]);
const newComment = ref("");
const commentLoading = ref(false);
const commentError = ref("");

function renderMd(text: string) {
  if (!text) return "";
  return marked.parse(text);
}

async function loadIssue() {
  if (!repoId.value) return;
  try {
    issue.value = await api.get(`/projects/${repoId.value}/issues/${issueNum}/`);
    comments.value = (await api.get(`/projects/${repoId.value}/issues/${issueNum}/comments/`)) || [];
  } catch (e: any) {
    error.value = e.message;
  }
}

async function toggleState() {
  if (!repoId.value || !issue.value) return;
  const newState = issue.value.state === "open" ? "closed" : "open";
  await api.patch(`/projects/${repoId.value}/issues/${issueNum}/`, { state: newState });
  issue.value.state = newState;
}

async function addComment() {
  if (!repoId.value) return;
  commentLoading.value = true;
  commentError.value = "";
  try {
    const c = await api.post(`/projects/${repoId.value}/issues/${issueNum}/comments/`, { body: newComment.value });
    comments.value.push(c);
    newComment.value = "";
  } catch (e: any) {
    commentError.value = e.message;
  }
  commentLoading.value = false;
}

import { watch } from "vue";
watch(repoId, loadIssue);
</script>
