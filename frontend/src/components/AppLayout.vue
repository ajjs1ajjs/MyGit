<template>
  <div class="flex h-screen overflow-hidden">
    <aside class="sidebar">
      <div class="logo">
        <RouterLink to="/" class="!text-white !no-underline">MyGit</RouterLink>
      </div>
      <nav class="flex-1 overflow-y-auto py-2">
        <RouterLink to="/" class="nav-item">
          🏠 Home
        </RouterLink>
        <RouterLink v-if="auth.user" :to="`/${auth.user.username}`" class="nav-item">
          👤 Your profile
        </RouterLink>
        <RouterLink to="/groups" class="nav-item">
          👥 Groups
        </RouterLink>
        <RouterLink to="/search" class="nav-item">
          🔍 Search
        </RouterLink>
        <template v-if="auth.user">
          <div class="divider"></div>
          <div class="section-label">Projects</div>
          <RouterLink v-for="r in repos" :key="r.id" :to="`/${r.path}`" class="nav-item truncate">
            <span class="w-4 text-center shrink-0">📁</span> {{ r.name }}
          </RouterLink>
        </template>
      </nav>
      <div class="p-4 border-t border-[#2b3138] text-xs">
        <template v-if="auth.user">
          <div class="text-white/80">{{ auth.user.username }}</div>
          <button @click="auth.logout()" class="text-white/50 hover:text-white mt-1">Sign out</button>
        </template>
        <template v-else>
          <RouterLink to="/auth/login" class="block text-blue-300 hover:text-white mb-1">Sign in</RouterLink>
          <RouterLink to="/auth/register" class="block text-blue-300 hover:text-white">Register</RouterLink>
        </template>
      </div>
    </aside>
    <div class="flex-1 flex flex-col overflow-hidden">
      <header class="topbar">
        <form @submit.prevent="doSearch" class="topbar-search">
          <input v-model="q" placeholder="Search or jump to..." />
        </form>
        <button v-if="auth.user" @click="showNew = true" class="btn btn-primary btn-sm">New project</button>
      </header>
      <main class="flex-1 overflow-y-auto p-6">
        <router-view />
      </main>
    </div>
    <div v-if="showNew" class="modal-overlay" @click.self="showNew=false">
      <div class="modal">
        <div class="card-header flex items-center justify-between">
          Create new project
          <button @click="showNew=false" class="text-gray-400 hover:text-gray-600">&times;</button>
        </div>
        <div class="card-body">
          <label class="text-xs font-semibold text-gray-500 block mb-1">Project name</label>
          <input v-model="newName" class="mb-3" @keyup.enter="create" />
          <label class="text-xs font-semibold text-gray-500 block mb-1">Visibility</label>
          <select v-model="newVis" class="mb-3">
            <option value="private">Private</option>
            <option value="public">Public</option>
            <option value="internal">Internal</option>
          </select>
          <div class="flex gap-2">
            <button @click="create" :disabled="!newName" class="btn btn-primary">Create project</button>
            <button @click="showNew=false" class="btn btn-primary-outline">Cancel</button>
          </div>
          <p v-if="newErr" class="text-red-500 text-xs mt-2">{{ newErr }}</p>
        </div>
      </div>
    </div>
    <Toast />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { api } from "../api/client";
import Toast from "./Toast.vue";

const route = useRoute(); const router = useRouter(); const auth = useAuthStore();
const q = ref(""); const repos = ref<any[]>([]);
const showNew = ref(false); const newName = ref(""); const newVis = ref("private"); const newErr = ref("");
function doSearch() { if (q.value.trim()) router.push(`/search?q=${encodeURIComponent(q.value.trim())}`); }
async function create() {
  if (!newName.value) return;
  try { const r = await api.post("/projects/", { name: newName.value, visibility: newVis.value }); showNew.value = false; newName.value = ''; router.push(`/${r.path}`); }
  catch (e: any) { newErr.value = e.message; }
}
onMounted(async () => { if (auth.token) { auth.fetchMe(); try { repos.value = ((await api.get("/projects/")) || []).slice(0, 15); } catch {} } });
</script>
