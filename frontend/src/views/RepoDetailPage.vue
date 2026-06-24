<template>
  <div v-if="repo">
    <div class="mb-4">
      <div class="flex items-center gap-2 flex-wrap">
        <h1 class="text-xl font-semibold text-[#24292f] dark:text-[#e6edf3]">{{ repo.name }}</h1>
        <span class="badge" :class="repo.visibility==='public'?'badge-info':'badge-warning'">{{ repo.visibility }}</span>
        <span v-if="repo.is_archived" class="badge badge-danger">archived</span>
      </div>
      <p v-if="repo.description" class="text-sm text-[#656d76] mt-1">{{ repo.description }}</p>
    </div>

    <div class="underline-nav">
      <RouterLink :to="`/${repo.path}`" :class="{ active: isTab('') }">Overview</RouterLink>
      <RouterLink :to="`/${repo.path}/-/tree/${repo.default_branch}`" :class="{ active: isTab('tree')||isTab('blob') }">Code</RouterLink>
      <RouterLink :to="`/${repo.path}/-/issues`" :class="{ active: isTab('issues')||isTab('issue') }">Issues</RouterLink>
      <RouterLink :to="`/${repo.path}/-/merge_requests`" :class="{ active: isTab('merge')||isTab('merge_request') }">Merge requests</RouterLink>
      <RouterLink :to="`/${repo.path}/-/commits/${repo.default_branch}`" :class="{ active: isTab('commit') }">Commits</RouterLink>
      <RouterLink :to="`/${repo.path}/-/branches`" :class="{ active: isTab('branches') }">Branches<span class="counter">{{ branches.length }}</span></RouterLink>
      <RouterLink :to="`/${repo.path}/-/tags`" :class="{ active: isTab('tags') }">Tags<span class="counter">{{ tags.length }}</span></RouterLink>
      <RouterLink :to="`/${repo.path}/-/wiki`" :class="{ active: isTab('wiki') }">Wiki</RouterLink>
      <RouterLink :to="`/${repo.path}/-/settings`" :class="{ active: isTab('settings') }">Settings</RouterLink>
    </div>

    <div v-if="isTab('')" class="flex gap-6 flex-col lg:flex-row">
      <div class="flex-1 min-w-0">
        <div class="card mb-4" v-if="commits.length">
          <div class="card-header">Recent commits</div>
          <div v-for="c in commits.slice(0,6)" :key="c.sha" class="flex items-center gap-3 px-5 py-2.5 border-b border-[#e1e4e8] dark:border-[#30363d] last:border-0 text-sm hover:bg-[#f6f8fa]">
            <RouterLink :to="`/${repo.path}/-/commit/${c.sha}`" class="font-mono text-xs text-[#656d76] w-[68px] shrink-0 hover:text-[#0969da]">{{ c.short_sha }}</RouterLink>
            <RouterLink :to="`/${repo.path}/-/commit/${c.sha}`" class="flex-1 truncate hover:text-[#0969da]">{{ c.message }}</RouterLink>
            <span class="text-xs text-[#656d76] shrink-0">{{ formatDate(c.committed_at) }}</span>
          </div>
        </div>
        <div class="card" v-if="tree.length">
          <div class="card-header flex items-center gap-2">
            Files <span class="font-normal font-mono text-xs text-[#656d76] bg-[#eaeef2] dark:bg-[#21262d] px-2 py-0.5 rounded-full">{{ repo.default_branch }}</span>
          </div>
          <div v-for="e in tree" :key="e.sha" class="flex items-center gap-2 px-5 py-2 border-b border-[#e1e4e8] dark:border-[#30363d] last:border-0 text-sm hover:bg-[#f6f8fa]">
            <span class="text-[#656d76]">{{ e.type==='tree'?'📁':'📄' }}</span>
            <RouterLink v-if="e.type==='tree'" :to="`/${repo.path}/-/tree/${repo.default_branch}/${e.path}`" class="text-[#0969da] hover:underline">{{ e.name }}</RouterLink>
            <span v-else class="text-[#24292f] dark:text-[#e6edf3]">{{ e.name }}</span>
          </div>
        </div>
      </div>
      <div class="w-full lg:w-[280px] shrink-0">
        <div class="card mb-4">
          <div class="card-body text-xs text-[#656d76] !p-3 space-y-2">
            <div class="font-semibold text-xs mb-2">Clone</div>
            <div class="bg-[#f6f8fa] dark:bg-[#0d1117] rounded-md p-2 font-mono break-all select-all text-xs">
              git clone {{ cloneUrl }}
            </div>
          </div>
        </div>
        <div class="card">
          <div class="card-body !p-3">
            <div class="font-semibold text-xs mb-2">About</div>
            <div class="grid grid-cols-2 gap-x-2 gap-y-1 text-xs">
              <span class="text-[#656d76]">Commits</span><span class="font-semibold">{{ commits.length }}</span>
              <span class="text-[#656d76]">Branches</span><span class="font-semibold">{{ branches.length }}</span>
              <span class="text-[#656d76]">Tags</span><span class="font-semibold">{{ tags.length }}</span>
              <span class="text-[#656d76]">Size</span><span class="font-semibold">{{ repo.size_kb>0?(repo.size_kb/1024).toFixed(1)+' MB':'0' }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <RouterView v-else />
  </div>
  <div v-else-if="loading" class="text-[#656d76]">Loading...</div>
  <div v-else class="text-[#cf222e]">{{ error||'Not found' }}</div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { api } from "../api/client";
import { useRepo } from "../composables/useRepo";

const route = useRoute(); const auth = useAuthStore();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const { repo, repoId, loading, error } = useRepo(repoUsername, repoName);
const commits = ref<any[]>([]); const branches = ref<any[]>([]); const tags = ref<any[]>([]); const tree = ref<any[]>([]);
const cloneUrl = computed(()=>`http://${window.location.host}/${repo.value?.path}.git`);
function isTab(n:string){return route.path.includes(`/-/${n}`)||(n===''&&!route.path.includes('/-/'))}
function formatDate(d:string){return d?new Date(d).toLocaleDateString('en-US',{month:'short',day:'numeric'}):''}
watch(repoId,async(id)=>{if(!id)return;try{const[c,b,t,tr]=await Promise.all([api.get(`/projects/${id}/commits/`),api.get(`/projects/${id}/branches/`),api.get(`/projects/${id}/tags/`),api.get(`/projects/${id}/tree/?ref=${repo.value?.default_branch||'main'}`)]);commits.value=c?.commits||[];branches.value=b?.branches||[];tags.value=t?.tags||[];tree.value=tr?.entries||[];}catch{}});
</script>
