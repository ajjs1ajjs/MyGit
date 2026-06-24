<template>
  <div v-if="repo">
    <!-- Project header -->
    <div class="mb-5">
      <div class="flex items-center gap-2 mb-1 flex-wrap">
        <h1 class="text-xl font-bold text-gray-800 dark:text-gray-200">{{ repo.name }}</h1>
        <span class="badge" :class="repo.visibility === 'public' ? 'badge-info' : 'badge-warning'">{{ repo.visibility }}</span>
        <span v-if="repo.is_archived" class="badge badge-danger">archived</span>
      </div>
      <p v-if="repo.description" class="text-sm text-gray-500 mt-0.5">{{ repo.description }}</p>
    </div>

    <!-- Tabs -->
    <div class="nav-tabs">
      <RouterLink :to="`/${repo.path}`" class="nav-tab" :class="{ active: isTab('') }">Project overview</RouterLink>
      <RouterLink :to="`/${repo.path}/-/tree/${repo.default_branch}`" class="nav-tab" :class="{ active: isTab('tree') }">Repository</RouterLink>
      <RouterLink :to="`/${repo.path}/-/commits/${repo.default_branch}`" class="nav-tab" :class="{ active: isTab('commits') || isTab('commit') }">Commits</RouterLink>
      <RouterLink :to="`/${repo.path}/-/branches`" class="nav-tab" :class="{ active: isTab('branches') }">Branches<span class="count">{{ branches.length }}</span></RouterLink>
      <RouterLink :to="`/${repo.path}/-/tags`" class="nav-tab" :class="{ active: isTab('tags') }">Tags<span class="count">{{ tags.length }}</span></RouterLink>
      <RouterLink :to="`/${repo.path}/-/issues`" class="nav-tab" :class="{ active: isTab('issues') || isTab('issue') }">Issues</RouterLink>
      <RouterLink :to="`/${repo.path}/-/merge_requests`" class="nav-tab" :class="{ active: isTab('merge_requests') || isTab('merge_request') }">Merge requests</RouterLink>
      <RouterLink :to="`/${repo.path}/-/wiki`" class="nav-tab" :class="{ active: isTab('wiki') }">Wiki</RouterLink>
      <RouterLink :to="`/${repo.path}/-/settings`" class="nav-tab" :class="{ active: isTab('settings') }">Settings</RouterLink>
    </div>

    <!-- Overview content -->
    <div v-if="isTab('')" class="flex gap-6 flex-col lg:flex-row">
      <div class="flex-1 min-w-0">
        <!-- recent commits -->
        <div class="card" v-if="commits.length">
          <div class="card-header">Recent commits</div>
          <div v-for="c in commits.slice(0,8)" :key="c.sha" class="flex items-center gap-3 py-2 border-b border-gray-100 dark:border-gray-800 last:border-0 text-sm">
            <span class="font-mono text-xs text-gray-400 w-[64px] shrink-0">{{ c.short_sha }}</span>
            <RouterLink :to="`/${repo.path}/-/commit/${c.sha}`" class="flex-1 text-gray-800 dark:text-gray-200 hover:text-blue-600 truncate no-underline">{{ c.message }}</RouterLink>
            <span class="text-xs text-gray-400 shrink-0">{{ formatDate(c.committed_at) }}</span>
          </div>
        </div>

        <!-- file browser -->
        <div class="card" v-if="tree.length">
          <div class="card-header">
            Files
            <span class="text-xs text-gray-400 font-normal ml-2">@{{ repo.default_branch }}</span>
          </div>
          <div v-for="e in tree" :key="e.sha" class="flex items-center gap-2 px-1 py-1.5 text-sm hover:bg-gray-50 dark:hover:bg-slate-800 rounded">
            <span>{{ e.type === "tree" ? "📁" : "📄" }}</span>
            <RouterLink v-if="e.type === 'tree'" :to="`/${repo.path}/-/tree/${repo.default_branch}/${e.path}`" class="text-blue-600 hover:underline">{{ e.name }}</RouterLink>
            <RouterLink v-else :to="`/${repo.path}/-/blob/${repo.default_branch}?path=${encodeURIComponent(e.path)}`" class="hover:text-blue-600">{{ e.name }}</RouterLink>
          </div>
          <RouterLink v-if="tree.length" :to="`/${repo.path}/-/tree/${repo.default_branch}`" class="text-xs text-blue-600 mt-2 inline-block">View all files</RouterLink>
        </div>
      </div>

      <!-- right sidebar -->
      <div class="w-full lg:w-[280px] shrink-0">
        <div class="card">
          <div class="text-xs text-gray-500 mb-2">Clone with SSH</div>
          <div class="bg-gray-100 dark:bg-slate-800 p-2 rounded text-xs font-mono break-all mb-2 select-all">
            git clone {{ cloneUrl }}
          </div>
          <div v-if="auth.token" class="text-xs text-gray-400">
            <div class="mb-1">Push an existing folder</div>
            <div class="bg-gray-100 dark:bg-slate-800 p-2 rounded text-xs font-mono break-all select-all">
              cd existing_folder<br/>
              git init --initial-branch={{ repo.default_branch }}<br/>
              git remote add origin {{ cloneUrl }}<br/>
              git add . && git commit -m "Initial commit"<br/>
              git push -u origin {{ repo.default_branch }}
            </div>
          </div>
        </div>
        <div class="card">
          <div class="card-header">Project information</div>
          <div class="grid grid-cols-2 gap-3 text-sm">
            <div class="text-gray-500">Commits</div><div class="font-semibold">{{ commits.length }}</div>
            <div class="text-gray-500">Branches</div><div class="font-semibold">{{ branches.length }}</div>
            <div class="text-gray-500">Tags</div><div class="font-semibold">{{ tags.length }}</div>
            <div class="text-gray-500">Size</div><div class="font-semibold">{{ repo.size_kb > 0 ? (repo.size_kb/1024).toFixed(1)+' MB' : '0' }}</div>
            <div class="text-gray-500">Created</div><div class="font-semibold">{{ formatDate(repo.created_at) }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Child routes -->
    <RouterView v-else />
  </div>
  <div v-else-if="loading" class="text-gray-500">Loading project...</div>
  <div v-else class="text-red-500">{{ error || 'Project not found' }}</div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { api } from "../api/client";
import { useRepo } from "../composables/useRepo";

const route = useRoute();
const auth = useAuthStore();
const repoUsername = route.params.username as string;
const repoName = route.params.repo as string;
const { repo, repoId, loading, error } = useRepo(repoUsername, repoName);
const commits = ref<any[]>([]);
const branches = ref<any[]>([]);
const tags = ref<any[]>([]);
const tree = ref<any[]>([]);

const cloneUrl = computed(() => `http://${window.location.host}/${repo.value?.path}.git`);

function isTab(name: string) {
  return route.path.includes(`/-/${name}`) || (name === '' && !route.path.includes('/-/'));
}

function formatDate(d: string) {
  if (!d) return '';
  return new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

watch(repoId, async (id) => {
  if (!id) return;
  try {
    const [c, b, t, tr] = await Promise.all([
      api.get(`/projects/${id}/commits/`),
      api.get(`/projects/${id}/branches/`),
      api.get(`/projects/${id}/tags/`),
      api.get(`/projects/${id}/tree/?ref=${repo.value?.default_branch || 'main'}`),
    ]);
    commits.value = c?.commits || [];
    branches.value = b?.branches || [];
    tags.value = t?.tags || [];
    tree.value = tr?.entries || [];
  } catch {}
});
</script>
