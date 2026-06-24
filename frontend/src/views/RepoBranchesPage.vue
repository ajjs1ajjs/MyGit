<template>
  <AppLayout>
    <div class="max-w-4xl mx-auto">
      <h2 class="font-semibold mb-3">Branches</h2>
      <div v-if="branches.length" class="border rounded divide-y">
        <div v-for="b in branches" :key="b.name" class="px-4 py-2.5 flex items-center justify-between text-sm hover:bg-gray-50 dark:hover:bg-slate-800">
          <span class="font-mono text-blue-600">{{ b.name }}</span>
          <div class="flex gap-3 text-xs text-gray-500">
            <span class="font-mono">{{ b.sha?.slice(0, 8) }}</span>
            <RouterLink :to="`/${repoUsername}/${repoName}/-/tree/${b.name}`" class="text-blue-600 hover:underline">Browse</RouterLink>
          </div>
        </div>
      </div>
      <p v-else-if="!loading" class="text-sm text-gray-500">No branches yet. Push commits to create branches.</p>
      <p v-if="loading" class="text-sm text-gray-500">Loading...</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import AppLayout from "../components/AppLayout.vue";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const { repoId, loading } = useRepo(repoUsername, repoName);
const branches = ref<any[]>([]);

watch(repoId, async (id) => {
  if (!id) return;
  try {
    const data = await api.get(`/projects/${id}/branches/`);
    branches.value = data?.branches || [];
  } catch {}
});
</script>
