<template>
  <div v-if="repo">
    <div class="mb-4">
      <div class="flex items-center gap-2 mb-1">
        <span class="text-gray-500">📁</span>
        <h1 class="text-xl font-bold">{{ repo.name }}</h1>
        <span class="badge" :class="repo.visibility === 'public' ? 'badge-info' : 'badge-warning'">{{ repo.visibility }}</span>
        <span v-if="repo.is_archived" class="badge badge-danger">archived</span>
      </div>
      <p v-if="repo.description" class="text-sm text-gray-500 mt-1">{{ repo.description }}</p>
    </div>

    <!-- Subnav -->
    <div class="flex gap-1 mb-5 border-b border-gray-200 dark:border-slate-700 text-sm">
      <RouterLink :to="`/${repo.path}`" class="px-3 py-2 border-b-2 -mb-[1px]" :class="isTab('') ? 'border-blue-600 text-blue-600 font-medium' : 'border-transparent text-gray-500 hover:text-gray-700'">Overview</RouterLink>
      <RouterLink :to="`/${repo.path}/-/tree/${repo.default_branch}`" class="px-3 py-2 border-b-2 -mb-[1px] border-transparent text-gray-500 hover:text-gray-700">Files</RouterLink>
      <RouterLink :to="`/${repo.path}/-/commits/${repo.default_branch}`" class="px-3 py-2 border-b-2 -mb-[1px] border-transparent text-gray-500 hover:text-gray-700">Commits</RouterLink>
      <RouterLink :to="`/${repo.path}/-/branches`" class="px-3 py-2 border-b-2 -mb-[1px] border-transparent text-gray-500 hover:text-gray-700">Branches</RouterLink>
      <RouterLink :to="`/${repo.path}/-/tags`" class="px-3 py-2 border-b-2 -mb-[1px] border-transparent text-gray-500 hover:text-gray-700">Tags</RouterLink>
      <RouterLink :to="`/${repo.path}/-/issues`" class="px-3 py-2 border-b-2 -mb-[1px] border-transparent text-gray-500 hover:text-gray-700">Issues</RouterLink>
      <RouterLink :to="`/${repo.path}/-/merge_requests`" class="px-3 py-2 border-b-2 -mb-[1px] border-transparent text-gray-500 hover:text-gray-700">Merge Requests</RouterLink>
      <RouterLink :to="`/${repo.path}/-/wiki`" class="px-3 py-2 border-b-2 -mb-[1px] border-transparent text-gray-500 hover:text-gray-700">Wiki</RouterLink>
      <RouterLink :to="`/${repo.path}/-/settings`" class="px-3 py-2 border-b-2 -mb-[1px] border-transparent text-gray-500 hover:text-gray-700">Settings</RouterLink>
    </div>

    <div v-if="isTab('')" class="max-w-5xl">
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-5">
        <div class="lg:col-span-2">
          <div class="card">
            <div class="flex items-center justify-between mb-3">
              <h3 class="font-semibold text-sm">Recent commits</h3>
              <RouterLink :to="`/${repo.path}/-/commits/${repo.default_branch}`" class="text-xs text-blue-600">View all</RouterLink>
            </div>
            <div v-if="commits.length">
              <RouterLink v-for="c in commits.slice(0,5)" :key="c.sha" :to="`/${repo.path}/-/commit/${c.sha}`" class="flex items-center gap-3 py-2 border-b last:border-0 text-sm no-underline hover:bg-gray-50 dark:hover:bg-slate-800 px-1 rounded">
                <span class="text-xs font-mono text-gray-400">{{ c.short_sha }}</span>
                <span class="flex-1 text-gray-700 dark:text-gray-300">{{ c.message }}</span>
                <span class="text-xs text-gray-400">{{ new Date(c.committed_at).toLocaleDateString() }}</span>
              </RouterLink>
            </div>
            <div v-else class="text-sm text-gray-500 py-2">No commits yet. Clone and push to get started.</div>
          </div>
        </div>
        <div>
          <div class="card mb-4">
            <h3 class="font-semibold text-sm mb-2">Stats</h3>
            <div class="grid grid-cols-2 gap-2 text-sm">
              <div class="border rounded p-2 text-center"><div class="font-bold">{{ commits.length }}</div><div class="text-xs text-gray-500">commits</div></div>
              <div class="border rounded p-2 text-center"><div class="font-bold">{{ branches.length }}</div><div class="text-xs text-gray-500">branches</div></div>
              <div class="border rounded p-2 text-center"><div class="font-bold">{{ tags.length }}</div><div class="text-xs text-gray-500">tags</div></div>
              <div class="border rounded p-2 text-center"><div class="font-bold">{{ repo.size_kb > 0 ? (repo.size_kb/1024).toFixed(1)+'MB' : '0KB' }}</div><div class="text-xs text-gray-500">size</div></div>
            </div>
          </div>
          <div class="card text-xs font-mono break-all">
            <div class="text-gray-400 mb-2">Clone</div>
            <div class="bg-gray-100 dark:bg-slate-800 p-2 rounded mb-2 select-all">
              git clone {{ cloneUrl }}
            </div>
            <div v-if="auth.token" class="text-gray-400 mb-2">Add remote & push</div>
            <div v-if="auth.token" class="bg-gray-100 dark:bg-slate-800 p-2 rounded select-all text-xs">
              git remote add origin {{ cloneUrl }}<br/>
              git push -u origin {{ repo.default_branch }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <RouterView v-else />
  </div>
  <div v-else-if="loading" class="text-gray-500">Loading...</div>
  <div v-else class="text-red-500">{{ error || 'Not found' }}</div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import { useRepo } from "../composables/useRepo";
import { useAuthStore } from "../stores/auth";

const route = useRoute();
const auth = useAuthStore();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const { repo, repoId, loading, error } = useRepo(repoUsername, repoName);
const commits = ref<any[]>([]);
const branches = ref<any[]>([]);
const tags = ref<any[]>([]);

const cloneUrl = computed(() => `http://${window.location.host}/${repo.value?.path}.git`);

function isTab(path: string) {
  const rp = route.path;
  if (!path) return rp === `/${repoUsername}/${repoName}` || rp.endsWith(`/${repoUsername}/${repoName}`);
  return rp.includes(`/-/${path}`);
}

watch(repoId, async (id) => {
  if (!id) return;
  try {
    const [c, b, t] = await Promise.all([
      api.get(`/projects/${id}/commits/`),
      api.get(`/projects/${id}/branches/`),
      api.get(`/projects/${id}/tags/`),
    ]);
    commits.value = c?.commits || [];
    branches.value = b?.branches || [];
    tags.value = t?.tags || [];
  } catch {}
});
</script>
