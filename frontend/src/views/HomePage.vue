<template>
  <div class="max-w-5xl">
    <h1 class="text-2xl font-bold mb-6">Projects</h1>

    <div v-if="auth.user" class="flex gap-3 mb-6">
      <button @click="showNew = true" class="btn btn-confirm">New project</button>
      <RouterLink to="/groups" class="btn btn-default">View groups</RouterLink>
    </div>

    <div v-if="!auth.user" class="card mb-6 flex items-center justify-between">
      <div>
        <h3 class="font-semibold mb-1">Welcome to MyGit</h3>
        <p class="text-sm text-gray-500">Self-hosted Git platform for your team</p>
      </div>
      <div class="flex gap-2">
        <RouterLink to="/auth/login" class="btn btn-confirm">Sign in</RouterLink>
        <RouterLink to="/auth/register" class="btn btn-default">Register</RouterLink>
      </div>
    </div>

    <div v-if="repos.length">
      <div class="nav-tabs mb-4">
        <button @click="filter = 'all'" class="nav-tab" :class="{ active: filter === 'all' }">All</button>
        <button @click="filter = 'yours'" v-if="auth.user" class="nav-tab" :class="{ active: filter === 'yours' }">Yours</button>
        <button @click="filter = 'starred'" class="nav-tab" :class="{ active: filter === 'starred' }">Starred</button>
      </div>
      <div v-if="filtered.length" class="grid gap-3">
        <RouterLink v-for="r in filtered" :key="r.id" :to="`/${r.path}`" class="card flex items-center gap-3 !mb-0 no-underline group">
          <span class="text-lg">📁</span>
          <div class="flex-1 min-w-0">
            <div class="font-semibold text-gray-800 dark:text-gray-200 group-hover:text-blue-600">{{ r.name }}</div>
            <div class="text-xs text-gray-400 mt-0.5">{{ r.path }} &middot; {{ formatDate(r.updated_at) }}</div>
          </div>
          <span class="badge" :class="r.visibility === 'public' ? 'badge-info' : 'badge-warning'">{{ r.visibility }}</span>
        </RouterLink>
      </div>
      <p v-else class="text-sm text-gray-500">No projects found.</p>
    </div>
    <p v-else-if="auth.user" class="text-sm text-gray-500">No projects yet. Create your first project!</p>

    <!-- New project modal -->
    <div v-if="showNew" class="fixed inset-0 bg-black/40 z-50 flex items-start justify-center pt-[10%]" @click.self="showNew=false">
      <div class="card w-full max-w-lg mx-4" style="border-radius:4px">
        <div class="card-header !text-base flex justify-between">New project <button @click="showNew=false" class="text-gray-400 hover:text-gray-600">&times;</button></div>
        <input v-model="newName" placeholder="Project name" class="mb-3" />
        <select v-model="newVis" class="mb-3"><option value="private">Private</option><option value="public">Public</option></select>
        <div class="flex gap-2"><button @click="create" :disabled="!newName" class="btn btn-confirm">Create</button><button @click="showNew=false" class="btn btn-default">Cancel</button></div>
        <p v-if="newErr" class="text-red-500 text-xs mt-2">{{ newErr }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { api } from "../api/client";
const router = useRouter(); const auth = useAuthStore();
const repos = ref<any[]>([]); const filter = ref("all");
const showNew = ref(false); const newName = ref(""); const newVis = ref("private"); const newErr = ref("");
const filtered = computed(() => {
  if (filter.value === 'yours' && auth.user) return repos.value.filter((r: any) => r.path?.startsWith(auth.user!.username + '/'));
  return repos.value;
});
async function create() {
  if (!newName.value) return;
  try { const r = await api.post("/projects/", { name: newName.value, visibility: newVis.value }); showNew.value = false; newName.value = ''; router.push(`/${r.path}`); }
  catch (e: any) { newErr.value = e.message; }
}
function formatDate(d: string) { return d ? new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }) : ''; }
onMounted(async () => {
  try { repos.value = (await api.get("/projects/")) || []; } catch {}
});
</script>
