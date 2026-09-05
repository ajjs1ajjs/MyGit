<template>
  <div class="max-w-2xl mx-auto">
      <h1 class="text-xl font-bold mb-6">Repository Settings</h1>
      <div class="bg-white dark:bg-slate-800 border rounded-lg p-4 mb-6" v-if="repo">
        <h3 class="font-semibold mb-3">{{ repo.path }}</h3>
        <form @submit.prevent="save" class="flex flex-col gap-3">
          <div>
            <label class="text-xs text-gray-500">Name</label>
            <input v-model="form.name" class="w-full px-3 py-2 border rounded text-sm" />
          </div>
          <div>
            <label class="text-xs text-gray-500">Description</label>
            <textarea v-model="form.description" rows="3" class="w-full px-3 py-2 border rounded text-sm"></textarea>
          </div>
          <div>
            <label class="text-xs text-gray-500">Visibility</label>
            <select v-model="form.visibility" class="w-full px-3 py-2 border rounded text-sm bg-white dark:bg-slate-800">
              <option value="public">Public</option>
              <option value="private">Private</option>
              <option value="internal">Internal</option>
            </select>
          </div>
          <p v-if="saveError" class="text-red-500 text-xs">{{ saveError }}</p>
          <p v-if="saveOk" class="text-green-600 text-xs">Saved.</p>
          <button type="submit" class="self-start px-4 py-2 bg-blue-600 text-white text-sm rounded hover:bg-blue-700">Save</button>
        </form>
      </div>

      <div class="border-t pt-6">
        <h3 class="font-semibold text-red-600 mb-2">Danger zone</h3>
        <p class="text-sm text-gray-500 mb-3">Once you delete a repository, there is no going back.</p>
        <button @click="confirmDelete = true" class="px-4 py-2 bg-red-600 text-white text-sm rounded hover:bg-red-700">Delete repository</button>
      </div>

      <div v-if="confirmDelete" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
        <div class="bg-white dark:bg-slate-800 rounded-lg p-6 max-w-sm mx-4 shadow-xl">
          <h3 class="font-semibold mb-2">Delete repository?</h3>
          <p class="text-sm text-gray-500 mb-4">Type <strong>{{ repo?.path }}</strong> to confirm.</p>
          <input v-model="deleteConfirm" class="w-full px-3 py-2 border rounded text-sm mb-4" :placeholder="repo?.path" />
          <div class="flex gap-2 justify-end">
            <button @click="confirmDelete = false" class="px-3 py-1.5 border rounded text-sm">Cancel</button>
            <button @click="doDelete" :disabled="deleteConfirm !== repo?.path" class="px-3 py-1.5 bg-red-600 text-white text-sm rounded disabled:opacity-50">Delete</button>
          </div>
        </div>
      </div>
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
const { repo, repoId } = useRepo(repoUsername, repoName);

const form = ref({ name: "", description: "", visibility: "private" });
const saveError = ref("");
const saveOk = ref(false);
const confirmDelete = ref(false);
const deleteConfirm = ref("");

watch(repo, (r) => {
  if (r) {
    form.value = { name: r.name, description: r.description || "", visibility: r.visibility };
  }
});

async function save() {
  if (!repoId.value) return;
  saveError.value = "";
  saveOk.value = false;
  try {
    await api.patch(`/projects/${repoId.value}/`, form.value);
    saveOk.value = true;
  } catch (e: any) {
    saveError.value = e.message;
  }
}

async function doDelete() {
  if (!repoId.value) return;
  try {
    await api.delete(`/projects/${repoId.value}/`);
    router.push(`/${repoUsername}`);
  } catch (e: any) {
    saveError.value = e.message;
  }
}
</script>
