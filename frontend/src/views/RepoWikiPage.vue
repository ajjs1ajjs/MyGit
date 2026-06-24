<template>
  <AppLayout>
    <div class="max-w-3xl mx-auto">
      <div class="flex items-center justify-between mb-4">
        <h2 class="font-semibold">Wiki</h2>
        <button @click="showNew = true" class="px-3 py-1.5 text-sm bg-blue-600 text-white rounded hover:bg-blue-700">New page</button>
      </div>

      <div v-if="pages.length" class="border rounded divide-y mb-6">
        <div v-for="p in pages" :key="p.slug" class="px-4 py-3 cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-800" @click="openPage(p)">
          <span class="text-sm font-medium">{{ p.title }}</span>
          <span class="text-xs text-gray-500 ml-2">by {{ p.author_username }}</span>
        </div>
      </div>
      <p v-else-if="!loading" class="text-sm text-gray-500">No wiki pages yet.</p>

      <div v-if="activePage" class="bg-white dark:bg-slate-800 border rounded-lg p-4">
        <div class="flex items-center justify-between mb-3">
          <h3 class="font-semibold">{{ activePage.title }}</h3>
          <button @click="activePage = null" class="text-sm text-gray-500 hover:text-gray-700">&times;</button>
        </div>
        <div class="prose dark:prose-invert max-w-none text-sm mb-4" v-html="renderMd(activePage.content)"></div>
        <button @click="editing = true" class="text-sm text-blue-600 hover:underline">Edit</button>
      </div>

      <div v-if="editing && activePage" class="bg-white dark:bg-slate-800 border rounded-lg p-4 mt-3">
        <h3 class="font-semibold mb-3">Edit: {{ activePage.slug }}</h3>
        <input v-model="editTitle" class="w-full px-3 py-2 border rounded text-sm mb-3" />
        <textarea v-model="editContent" rows="10" class="w-full px-3 py-2 border rounded text-sm mb-3"></textarea>
        <div class="flex gap-2">
          <button @click="savePage" class="px-3 py-1.5 text-sm bg-blue-600 text-white rounded">Save</button>
          <button @click="editing = false" class="px-3 py-1.5 text-sm border rounded">Cancel</button>
        </div>
      </div>

      <div v-if="showNew" class="bg-white dark:bg-slate-800 border rounded-lg p-4 mt-3">
        <h3 class="font-semibold mb-3">New wiki page</h3>
        <input v-model="newSlug" placeholder="slug" class="w-full px-3 py-2 border rounded text-sm mb-3" />
        <input v-model="newTitle" placeholder="Title" class="w-full px-3 py-2 border rounded text-sm mb-3" />
        <textarea v-model="newContent" rows="8" placeholder="Content (Markdown)" class="w-full px-3 py-2 border rounded text-sm mb-3"></textarea>
        <div class="flex gap-2">
          <button @click="createPage" :disabled="!newSlug" class="px-3 py-1.5 text-sm bg-blue-600 text-white rounded disabled:opacity-50">Create</button>
          <button @click="showNew = false" class="px-3 py-1.5 text-sm border rounded">Cancel</button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import { marked } from "marked";
import AppLayout from "../components/AppLayout.vue";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const { repoId } = useRepo(repoUsername, repoName);

const pages = ref<any[]>([]);
const loading = ref(true);
const activePage = ref<any>(null);
const showNew = ref(false);
const newSlug = ref("");
const newTitle = ref("");
const newContent = ref("");
const editing = ref(false);
const editTitle = ref("");
const editContent = ref("");

function renderMd(text: string) {
  if (!text) return "";
  return marked.parse(text);
}

async function loadPages() {
  if (!repoId.value) return;
  loading.value = true;
  try {
    pages.value = (await api.get(`/projects/${repoId.value}/wiki/`)) || [];
  } catch {}
  loading.value = false;
}

function openPage(p: any) {
  activePage.value = p;
  editTitle.value = p.title;
  editContent.value = p.content;
  editing.value = false;
}

async function createPage() {
  if (!repoId.value || !newSlug.value) return;
  try {
    await api.post(`/projects/${repoId.value}/wiki/`, {
      slug: newSlug.value,
      title: newTitle.value || newSlug.value,
      content: newContent.value,
    });
    showNew.value = false;
    newSlug.value = "";
    newTitle.value = "";
    newContent.value = "";
    await loadPages();
  } catch {}
}

async function savePage() {
  if (!repoId.value || !activePage.value) return;
  try {
    await api.put(`/projects/${repoId.value}/wiki/${activePage.value.slug}/`, {
      title: editTitle.value,
      content: editContent.value,
    });
    editing.value = false;
    await loadPages();
  } catch {}
}

watch(repoId, loadPages);
</script>
