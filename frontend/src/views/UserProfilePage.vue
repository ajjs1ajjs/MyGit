<template>
  <div class="max-w-4xl mx-auto">
      <div class="flex items-center gap-4 mb-6">
        <div class="w-16 h-16 rounded-full bg-blue-500 flex items-center justify-center text-white text-xl font-bold">{{ initials }}</div>
        <div>
          <h1 class="text-2xl font-bold">{{ profile?.full_name || username }}</h1>
          <p class="text-sm text-gray-500">{{ username }}</p>
          <p v-if="profile?.bio" class="text-sm text-gray-600 mt-1">{{ profile.bio }}</p>
        </div>
      </div>

      <h2 class="font-semibold mb-3">Repositories</h2>
      <div v-if="repos.length" class="grid gap-3">
        <RouterLink v-for="r in repos" :key="r.id" :to="`/${r.path}`" class="p-4 border rounded-lg hover:shadow block">
          <div class="flex items-center justify-between">
            <h3 class="font-semibold">{{ r.name }}</h3>
            <span class="text-xs px-2 py-0.5 rounded bg-gray-200 dark:bg-slate-700">{{ r.visibility }}</span>
          </div>
          <p class="text-sm text-gray-500 mt-1">{{ r.description || r.path }}</p>
          <div class="text-xs text-gray-400 mt-2">{{ new Date(r.updated_at).toLocaleDateString() }}</div>
        </RouterLink>
      </div>
      <p v-else-if="!loading" class="text-gray-500 text-sm">No repositories yet.</p>
      <p v-if="loading" class="text-sm text-gray-500">Loading...</p>
    </div>
  </template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
const route = useRoute();
const username = route.params.username as string;
const profile = ref<any>(null);
const repos = ref<any[]>([]);
const loading = ref(true);

const initials = computed(() => {
  const name = profile.value?.full_name || profile.value?.username || username;
  return name.slice(0, 2).toUpperCase();
});

onMounted(async () => {
  try {
    const [profData, repoData] = await Promise.all([
      api.get(`/users/${username}/`),
      api.get("/projects/"),
    ]);
    profile.value = profData;
    repos.value = (repoData || []).filter((r: any) => r.path?.startsWith(username + "/"));
  } catch {}
  loading.value = false;
});
</script>
