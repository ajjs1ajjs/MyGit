<template>
  <div v-if="repo">
    <div class="flex items-center gap-2 mb-5 flex-wrap">
      <h1 class="text-xl font-semibold">{{ repo.name }}</h1>
      <span class="badge" :class="repo.visibility==='public'?'badge-blue':'badge-gray'">{{ repo.visibility }}</span>
      <span v-if="repo.is_archived" class="badge badge-red">archived</span>
    </div>

    <div class="navtabs">
      <RouterLink :to="`/${repo.path}`" :class="{active:isTab('')}">Overview</RouterLink>
      <RouterLink :to="`/${repo.path}/-/tree/${repo.default_branch}`" :class="{active:isTab('tree')||isTab('blob')}">Code</RouterLink>
      <RouterLink :to="`/${repo.path}/-/issues`" :class="{active:isTab('issues')||isTab('issue')}">Issues</RouterLink>
      <RouterLink :to="`/${repo.path}/-/merge_requests`" :class="{active:isTab('merge')}">Merge requests</RouterLink>
      <RouterLink :to="`/${repo.path}/-/commits/${repo.default_branch}`" :class="{active:isTab('commit')}">Commits</RouterLink>
      <RouterLink :to="`/${repo.path}/-/branches`" :class="{active:isTab('branches')}">Branches<span class="counter">{{ branches.length }}</span></RouterLink>
      <RouterLink :to="`/${repo.path}/-/tags`" :class="{active:isTab('tags')}">Tags<span class="counter">{{ tags.length }}</span></RouterLink>
      <RouterLink :to="`/${repo.path}/-/wiki`" :class="{active:isTab('wiki')}">Wiki</RouterLink>
      <RouterLink :to="`/${repo.path}/-/settings`" :class="{active:isTab('settings')}">Settings</RouterLink>
    </div>

    <div v-if="isTab('')" class="flex gap-6 flex-col lg:flex-row">
      <div class="flex-1">
        <div class="card mb-4">
          <div class="card-header">Recent commits</div>
          <div v-if="commits.length">
            <div v-for="c in commits.slice(0,6)" :key="c.sha" class="flex items-center gap-3 px-5 py-2.5 border-b last:border-0 text-sm border-[#dee2e6] dark:border-[#2a2a4a]">
              <RouterLink :to="`/${repo.path}/-/commit/${c.sha}`" class="font-mono text-xs text-[#6c757d] w-[64px] shrink-0">{{ c.short_sha }}</RouterLink>
              <RouterLink :to="`/${repo.path}/-/commit/${c.sha}`" class="flex-1 truncate">{{ c.message }}</RouterLink>
              <span class="text-xs text-[#6c757d] shrink-0">{{ fmt(c.committed_at) }}</span>
            </div>
          </div>
          <div v-else class="px-5 py-4 text-sm text-[#6c757d]">No commits yet</div>
        </div>
        <div class="card">
          <div class="card-header flex items-center gap-2">Files <span class="font-normal text-xs text-[#6c757d] bg-[#e9ecef] px-2 py-0.5 rounded-full">{{ repo.default_branch }}</span></div>
          <div v-if="tree.length">
            <div v-for="e in tree" :key="e.sha" class="flex items-center gap-2 px-5 py-2 border-b last:border-0 text-sm border-[#dee2e6] dark:border-[#2a2a4a]">
              <span class="text-[#6c757d] w-5 text-center">{{ e.type==='tree'?'📁':'📄' }}</span>
              <RouterLink v-if="e.type==='tree'" :to="`/${repo.path}/-/tree/${repo.default_branch}/${e.path}`" class="text-[#4263eb]">{{ e.name }}</RouterLink>
              <span v-else class="text-[#212529] dark:text-white">{{ e.name }}</span>
            </div>
          </div>
          <div v-else class="px-5 py-4 text-sm text-[#6c757d]">Empty repository</div>
        </div>
      </div>
      <div class="w-full lg:w-[260px]">
        <div class="card mb-4"><div class="card-body !p-4 text-xs"><div class="font-semibold text-xs mb-2">Clone</div><div class="bg-[#f8f9fa] dark:bg-[#0f172a] rounded p-2 font-mono text-xs break-all select-all">{{ cloneUrl }}</div></div></div>
        <div class="card"><div class="card-body !p-4"><div class="font-semibold text-xs mb-2">About</div><div class="grid grid-cols-2 gap-1 text-xs"><span class="text-[#6c757d]">Commits</span><span class="font-semibold">{{ commits.length }}</span><span class="text-[#6c757d]">Branches</span><span class="font-semibold">{{ branches.length }}</span><span class="text-[#6c757d]">Tags</span><span class="font-semibold">{{ tags.length }}</span><span class="text-[#6c757d]">Size</span><span class="font-semibold">{{ repo.size_kb>0?(repo.size_kb/1024).toFixed(1)+'MB':'0' }}</span></div></div></div>
      </div>
    </div>
    <RouterView v-else />
  </div>
  <div v-else-if="loading" class="text-[#6c757d]">Loading...</div>
  <div v-else class="text-[#e03131]">{{ error||'Not found' }}</div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue"; import { useRoute } from "vue-router"; import { api } from "../api/client"; import { useRepo } from "../composables/useRepo";
const route = useRoute();
const repoUsername = route.params.username as string; const repoName = route.params.repo as string;
const { repo, repoId, loading, error } = useRepo(repoUsername, repoName);
const commits = ref<any[]>([]); const branches = ref<any[]>([]); const tags = ref<any[]>([]); const tree = ref<any[]>([]);
const cloneUrl = computed(() => `http://${window.location.host}/${repo.value?.path}.git`);
function isTab(n: string) { return route.path.includes(`/-/${n}`) || (n === '' && !route.path.includes('/-/')) }
function fmt(d: string) { return d ? new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) : '' }
watch(repoId, async (id) => { if (!id) return; try { const [c, b, t, tr] = await Promise.all([api.get(`/projects/${id}/commits/`), api.get(`/projects/${id}/branches/`), api.get(`/projects/${id}/tags/`), api.get(`/projects/${id}/tree/?ref=${repo.value?.default_branch || 'main'}`)]); commits.value = c?.commits || []; branches.value = b?.branches || []; tags.value = t?.tags || []; tree.value = tr?.entries || []; } catch {} });
</script>
