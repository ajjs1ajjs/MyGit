<template>
  <div v-if="repo">
    <div class="flex items-center gap-3 mb-5">
      <h1 class="text-lg font-semibold tracking-tight">{{ repo.name }}</h1>
      <span class="badge" :class="repo.visibility==='public'?'badge-blue':'badge-gray'">{{ repo.visibility }}</span>
      <span v-if="repo.is_archived" class="badge badge-orange">archived</span>
    </div>
    <div class="navtabs">
      <RouterLink :to="`/${repo.path}`" exact-active-class="active">Overview</RouterLink>
      <RouterLink :to="`/${repo.path}/-/tree/${repo.default_branch}`" active-class="active">Code</RouterLink>
      <RouterLink :to="`/${repo.path}/-/issues`" active-class="active">Issues</RouterLink>
      <RouterLink :to="`/${repo.path}/-/merge_requests`" active-class="active">Merge requests</RouterLink>
      <RouterLink :to="`/${repo.path}/-/commits/${repo.default_branch}`" active-class="active">Commits</RouterLink>
      <RouterLink :to="`/${repo.path}/-/branches`" active-class="active">Branches <span v-if="branches.length" class="counter">{{branches.length}}</span></RouterLink>
      <RouterLink :to="`/${repo.path}/-/tags`" active-class="active">Tags <span v-if="tags.length" class="counter">{{tags.length}}</span></RouterLink>
      <RouterLink :to="`/${repo.path}/-/wiki`" active-class="active">Wiki</RouterLink>
      <RouterLink :to="`/${repo.path}/-/settings`" active-class="active">Settings</RouterLink>
    </div>
    <router-view v-if="route.matched.length > 1" />
    <div v-else class="flex gap-6 flex-col lg:flex-row">
      <div class="flex-1 min-w-0">
        <div class="card mb-4">
          <div class="card-header">Recent commits</div>
          <div v-if="commits.length" class="divide-y">
            <RouterLink v-for="c in commits.slice(0,8)" :key="c.sha" :to="`/${repo.path}/-/commit/${c.sha}`" class="flex items-center gap-3 px-5 py-2.5 text-sm hover:bg-[#f5f5f5] dark:hover:bg-[#1a1a1a] !no-underline">
              <span class="font-mono text-xs text-[#737373] w-[72px] shrink-0">{{c.short_sha}}</span>
              <span class="flex-1 truncate">{{c.message}}</span>
              <span class="text-xs text-[#a3a3a3] shrink-0">{{fmt(c.committed_at)}}</span>
            </RouterLink>
          </div>
          <div v-else class="empty-state"><div class="icon">&#128194;</div><h3>No commits yet</h3><p>Push your first commit to get started.</p></div>
        </div>
        <div class="card">
          <div class="card-header">Files <span class="font-normal text-xs text-[#a3a3a3] bg-[#f3f4f6] dark:bg-[#262626] px-2 py-0.5 rounded-full font-mono">{{repo.default_branch}}</span></div>
          <div v-if="tree.length" class="divide-y">
            <div v-for="e in tree" :key="e.sha" class="flex items-center gap-3 px-5 py-2.5 text-sm hover:bg-[#f5f5f5] dark:hover:bg-[#1a1a1a]">
              <span class="text-[#a3a3a3] w-4 text-center">{{e.type==='tree'?'📁':'📄'}}</span>
              <RouterLink v-if="e.type==='tree'" :to="`/${repo.path}/-/tree/${repo.default_branch}/${e.path}`" class="text-[#2563eb] hover:underline truncate">{{e.name}}</RouterLink>
              <span v-else class="truncate">{{e.name}}</span>
            </div>
          </div>
          <div v-else class="empty-state"><div class="icon">&#128229;</div><h3>Empty repository</h3><p>Clone and push to populate.</p></div>
        </div>
      </div>
      <div class="w-[252px] shrink-0 max-lg:w-full space-y-4">
        <div class="card"><div class="card-body !p-4 text-xs"><div class="font-semibold text-xs mb-2">Clone</div><div class="bg-[#f5f5f5] dark:bg-[#0a0a0a] rounded p-2.5 font-mono break-all select-all leading-relaxed">{{cloneUrl}}</div></div></div>
        <div class="card"><div class="card-body !p-4"><div class="font-semibold text-xs mb-3">About</div><div class="grid grid-cols-2 gap-x-2 gap-y-1.5 text-xs"><span class="text-[#737373]">Commits</span><span>{{commits.length}}</span><span class="text-[#737373]">Branches</span><span>{{branches.length}}</span><span class="text-[#737373]">Tags</span><span>{{tags.length}}</span><span class="text-[#737373]">Size</span><span>{{repo.size_kb>0?(repo.size_kb/1024).toFixed(1)+' MB':'0'}}</span></div></div></div>
        <div class="card" v-if="repo.description"><div class="card-body !p-4"><div class="font-semibold text-xs mb-1">Description</div><p class="text-xs text-[#737373]">{{repo.description}}</p></div></div>
      </div>
    </div>
  </div>
  <div v-else-if="loading" class="space-y-3"><div class="skeleton h-7 w-48"></div><div class="skeleton h-9 w-full max-w-lg"></div><div class="skeleton h-64 w-full"></div></div>
  <div v-else class="empty-state"><div class="icon">&#128533;</div><h3>{{error||'Project not found'}}</h3></div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue"; import { useRoute } from "vue-router"; import { api } from "../api/client"; import { useRepo } from "../composables/useRepo";
const route = useRoute(); const repoUsername = route.params.username as string; const repoName = route.params.repo as string;
const { repo, repoId, loading, error } = useRepo(repoUsername, repoName);
const commits = ref<any[]>([]); const branches = ref<any[]>([]); const tags = ref<any[]>([]); const tree = ref<any[]>([]);
const cloneUrl = computed(() => `http://${window.location.host}/${repo.value?.path}.git`);
function fmt(d: string) { return d ? new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) : ''; }
watch(repoId, async (id) => { if (!id) return; try { const[c,b,t,tr]=await Promise.all([api.get(`/projects/${id}/commits/`),api.get(`/projects/${id}/branches/`),api.get(`/projects/${id}/tags/`),api.get(`/projects/${id}/tree/?ref=${repo.value?.default_branch||'main'}`)]); commits.value=c||[]; branches.value=b||[]; tags.value=t||[]; tree.value=tr||[]; } catch {} });
</script>
