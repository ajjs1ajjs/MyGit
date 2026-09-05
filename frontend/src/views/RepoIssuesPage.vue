<template>
  <div class="max-w-5xl">
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold">Issues</h2>
      <RouterLink :to="`/${repoUsername}/${repoName}/-/issues/new`" class="btn btn-confirm btn-sm">New issue</RouterLink>
    </div>
    <div v-if="issues.length" class="card !p-0 !mb-0">
      <div class="border-b border-gray-100 dark:border-gray-800 px-4 py-2 flex gap-3 text-xs text-gray-500">
        <button @click="stateFilter = 'open'" :class="stateFilter==='open' ? 'font-semibold text-gray-800' : ''">Open</button>
        <button @click="stateFilter = 'closed'" :class="stateFilter==='closed' ? 'font-semibold text-gray-800' : ''">Closed</button>
        <button @click="stateFilter = ''" :class="!stateFilter ? 'font-semibold text-gray-800' : ''">All</button>
      </div>
      <RouterLink v-for="i in filtered" :key="i.number" :to="`/${repoUsername}/${repoName}/-/issues/${i.number}`" class="flex items-center gap-3 px-4 py-3 border-b border-gray-50 dark:border-gray-800 last:border-0 hover:bg-gray-50 dark:hover:bg-slate-800 no-underline">
        <span :class="i.state==='open'?'text-green-500':'text-red-500'" class="text-lg leading-none shrink-0">&bull;</span>
        <div class="flex-1 min-w-0">
          <span class="text-sm text-gray-800 dark:text-gray-200">{{ i.title }}</span>
          <div class="text-xs text-gray-500 mt-0.5">#{{ i.number }} &middot; {{ i.author_username }} &middot; {{ formatDate(i.created_at) }}</div>
        </div>
        <div v-if="i.labels?.length" class="flex gap-1 shrink-0">
          <span v-for="l in i.labels" :key="l.id" :style="{ background: l.color + '20', color: l.color }" class="px-2 py-0.5 text-xs rounded-full">{{ l.name }}</span>
        </div>
      </RouterLink>
    </div>
    <p v-else-if="!loading" class="text-sm text-gray-500">No issues yet.</p>
    <p v-if="loading" class="text-sm text-gray-500">Loading...</p>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const { repoId, loading } = useRepo(repoUsername, repoName);
const issues = ref<any[]>([]);
const stateFilter = ref("open");
const filtered = computed(() => stateFilter.value ? issues.value.filter((i: any) => i.state === stateFilter.value) : issues.value);

function formatDate(d: string) { return d ? new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) : ''; }

watch(repoId, async (id) => { if (!id) return; try { issues.value = (await api.get(`/projects/${id}/issues/`)) || []; } catch {} });
</script>
