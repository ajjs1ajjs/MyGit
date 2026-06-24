<template>
  <div v-if="repo">
    <div class="flex items-center gap-2 mb-4 flex-wrap">
      <svg class="icon text-[#6c757d]" viewBox="0 0 24 24"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
      <h1 class="text-lg font-semibold">{{ repo.name }}</h1>
      <span class="badge" :class="repo.visibility==='public'?'badge-blue':'badge-gray'">{{ repo.visibility }}</span>
    </div>

    <div class="navtabs">
      <RouterLink :to="`/${repo.path}`" exactActiveClass="active">Overview</RouterLink>
      <RouterLink :to="`/${repo.path}/-/tree/${repo.default_branch}`" activeClass="active">Code</RouterLink>
      <RouterLink :to="`/${repo.path}/-/issues`" activeClass="active">Issues</RouterLink>
      <RouterLink :to="`/${repo.path}/-/merge_requests`" activeClass="active">Merge requests</RouterLink>
      <RouterLink :to="`/${repo.path}/-/commits/${repo.default_branch}`" activeClass="active">Commits</RouterLink>
      <RouterLink :to="`/${repo.path}/-/branches`" activeClass="active">Branches <span class="counter">{{branches.length}}</span></RouterLink>
      <RouterLink :to="`/${repo.path}/-/tags`" activeClass="active">Tags <span class="counter">{{tags.length}}</span></RouterLink>
      <RouterLink :to="`/${repo.path}/-/wiki`" activeClass="active">Wiki</RouterLink>
      <RouterLink :to="`/${repo.path}/-/settings`" activeClass="active">Settings</RouterLink>
    </div>

    <router-view v-if="hasChild" />
    <div v-else class="flex gap-6 flex-col lg:flex-row">
      <div class="flex-1">
        <div class="card mb-4">
          <div class="card-header">Recent commits</div>
          <div v-if="commits.length" class="divide-y">
            <RouterLink v-for="c in commits.slice(0,6)" :key="c.sha" :to="`/${repo.path}/-/commit/${c.sha}`" class="flex items-center gap-3 px-5 py-2.5 text-sm hover:bg-[#f8f9fa] !no-underline">
              <span class="font-mono text-xs text-[#6c757d] w-[68px] shrink-0">{{c.short_sha}}</span>
              <span class="flex-1 truncate text-[#212529] dark:text-white">{{c.message}}</span>
              <span class="text-xs text-[#6c757d] shrink-0">{{fmt(c.committed_at)}}</span>
            </RouterLink>
          </div>
          <div v-else class="px-5 py-8 text-center text-sm text-[#6c757d]">
            <div class="text-2xl mb-2">📂</div>
            No commits yet. Push your first commit!
          </div>
        </div>
        <div class="card">
          <div class="card-header flex items-center gap-2">
            Files
            <span class="font-normal text-xs text-[#6c757d] bg-[#e9ecef] px-2 py-0.5 rounded-full">{{repo.default_branch}}</span>
          </div>
          <div v-if="tree.length" class="divide-y">
            <div v-for="e in tree" :key="e.sha" class="flex items-center gap-2 px-5 py-2 text-sm hover:bg-[#f8f9fa]">
              <span class="text-[#6c757d] w-5 text-center">{{e.type==='tree'?'📁':'📄'}}</span>
              <RouterLink v-if="e.type==='tree'" :to="`/${repo.path}/-/tree/${repo.default_branch}/${e.path}`" class="text-[#4263eb] hover:underline">{{e.name}}</RouterLink>
              <span v-else class="text-[#212529] dark:text-white">{{e.name}}</span>
            </div>
          </div>
          <div v-else class="px-5 py-8 text-center text-sm text-[#6c757d]">Empty repository</div>
        </div>
      </div>
      <div class="w-[260px] shrink-0 max-lg:w-full">
        <div class="card mb-4">
          <div class="p-4 text-xs">
            <div class="font-semibold text-xs mb-2">Clone</div>
            <div class="bg-[#f8f9fa] dark:bg-[#0f172a] rounded p-2 font-mono break-all select-all">{{cloneUrl}}</div>
          </div>
        </div>
        <div class="card">
          <div class="p-4">
            <div class="font-semibold text-xs mb-2">About</div>
            <div class="grid grid-cols-2 gap-x-2 gap-y-1 text-xs">
              <span class="text-[#6c757d]">Commits</span><span class="font-semibold">{{commits.length}}</span>
              <span class="text-[#6c757d]">Branches</span><span class="font-semibold">{{branches.length}}</span>
              <span class="text-[#6c757d]">Tags</span><span class="font-semibold">{{tags.length}}</span>
              <span class="text-[#6c757d]">Size</span><span class="font-semibold">{{repo.size_kb>0?(repo.size_kb/1024).toFixed(1)+'MB':'0'}}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
  <div v-else-if="loading" class="text-[#6c757d]">Loading...</div>
  <div v-else class="text-[#e03131]">{{error||'Not found'}}</div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const { repo, repoId, loading, error } = useRepo(repoUsername, repoName);
const commits = ref<any[]>([]); const branches = ref<any[]>([]); const tags = ref<any[]>([]); const tree = ref<any[]>([]);

const cloneUrl = computed(() => `http://${window.location.host}/${repo.value?.path}.git`);
const hasChild = computed(() => route.matched.length > 1);

function fmt(d: string) { return d ? new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) : ''; }

watch(repoId, async (id) => {
  if (!id) return;
  try {
    const [c, b, t, tr] = await Promise.all([
      api.get(`/projects/${id}/commits/`),
      api.get(`/projects/${id}/branches/`),
      api.get(`/projects/${id}/tags/`),
      api.get(`/projects/${id}/tree/?ref=${repo.value?.default_branch || 'main'}`),
    ]);
    commits.value = c || []; branches.value = b || []; tags.value = t || []; tree.value = tr || [];
  } catch {}
});
</script>
