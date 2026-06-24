<template>
  <AppLayout>
    <div class="max-w-5xl mx-auto">
      <div class="mb-3 flex items-center gap-2 text-sm">
        <RouterLink :to="`/${repoUsername}/${repoName}`" class="text-blue-600 hover:underline">{{ repoUsername }}/{{ repoName }}</RouterLink>
        <span class="text-gray-400">/</span>
        <span>{{ filePath }}</span>
      </div>
      <div class="bg-white dark:bg-slate-900 border rounded-lg overflow-hidden">
        <div class="px-4 py-2 bg-gray-100 dark:bg-slate-800 border-b text-xs font-mono text-gray-500 flex justify-between">
          <span>{{ filePath }} ({{ lines }} lines)</span>
          <span>{{ content?.length || 0 | 0 }} bytes</span>
        </div>
        <div class="overflow-x-auto">
          <table v-if="content" class="w-full text-sm font-mono">
            <tbody>
              <tr v-for="(line, i) in contentLines" :key="i" class="hover:bg-gray-50 dark:hover:bg-slate-800">
                <td class="pl-4 pr-3 py-0.5 text-right text-xs text-gray-400 select-none border-r w-12">{{ i + 1 }}</td>
                <td class="pl-4 py-0.5 whitespace-pre-wrap break-words"><code>{{ line }}</code></td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <p v-if="loading" class="text-gray-500 mt-4 text-sm">Loading...</p>
      <p v-if="error" class="text-red-500 mt-4 text-sm">{{ error }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import AppLayout from "../components/AppLayout.vue";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const refParam = ((route.params.ref as string) || "main");
const filePath = (route.query.path as string) || "";
const { repoId, loading, error } = useRepo(repoUsername, repoName);

const content = ref("");
const lines = ref(0);

const contentLines = computed(() => content.value.split("\n"));

watch(repoId, async (id) => {
  if (!id) return;
  try {
    const sha = filePath ? "0" : refParam;
    let url = `/projects/${id}/blobs/${sha}/?ref=${refParam}`;
    if (filePath) url += `&path=${encodeURIComponent(filePath)}`;
    const data = await api.get(url);
    content.value = data?.content || "";
    lines.value = content.value.split("\n").length;
  } catch (e: any) {
    error.value = e.message;
  }
});
</script>
