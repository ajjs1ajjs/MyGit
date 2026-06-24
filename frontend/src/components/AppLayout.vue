<template>
  <div class="flex h-screen overflow-hidden">
    <!-- Sidebar -->
    <aside class="sidebar w-[220px] flex flex-col shrink-0">
      <div class="flex items-center gap-2 px-4 h-[48px] border-b border-white/5">
        <span class="text-white font-semibold text-base">MyGit</span>
      </div>
      <nav class="flex-1 overflow-y-auto py-2">
        <RouterLink to="/" class="nav-item" :class="{ 'router-link-exact-active': route.path === '/' }">
          <span class="nav-icon">🏠</span> Home
        </RouterLink>

        <div v-if="auth.user" class="section-title">Projects</div>
        <RouterLink v-if="auth.user" :to="`/${auth.user.username}`" class="nav-item">
          <span class="nav-icon">👤</span> Your work
        </RouterLink>
        <RouterLink to="/groups" class="nav-item">
          <span class="nav-icon">👥</span> Groups
        </RouterLink>
        <RouterLink to="/search" class="nav-item">
          <span class="nav-icon">🔍</span> Search
        </RouterLink>

        <div v-if="repos.length" class="section-title">Recent projects</div>
        <RouterLink v-for="r in repos" :key="r.id" :to="`/${r.path}`" class="nav-item truncate">
          <span class="nav-icon">📁</span> {{ r.name }}
        </RouterLink>
      </nav>
      <div class="px-4 py-3 border-t border-white/5 text-xs">
        <template v-if="auth.user">
          <div class="text-gray-400">{{ auth.user.username }}</div>
          <button @click="auth.logout()" class="text-red-400 hover:text-red-300 mt-0.5">Sign out</button>
        </template>
        <template v-else>
          <RouterLink to="/auth/login" class="block mb-1 text-blue-300 hover:text-white">Sign in</RouterLink>
          <RouterLink to="/auth/register" class="block text-blue-300 hover:text-white">Register</RouterLink>
        </template>
      </div>
    </aside>

    <!-- Main area -->
    <div class="flex-1 flex flex-col overflow-hidden">
      <header class="topbar">
        <form @submit.prevent="doSearch" class="topbar-search">
          <input v-model="q" placeholder="Search or jump to..." class="w-full" />
        </form>
        <template v-if="auth.user">
          <button @click="showNewProject = true" class="btn btn-confirm btn-sm">+ New project</button>
        </template>
      </header>
      <main class="flex-1 overflow-y-auto p-6">
        <router-view />
      </main>
    </div>

    <!-- New project modal -->
    <div v-if="showNewProject" class="fixed inset-0 bg-black/40 z-50 flex items-start justify-center pt-[10%]" @click.self="showNewProject=false">
      <div class="card w-full max-w-lg mx-4" style="border-radius:4px">
        <div class="card-header !text-base flex items-center justify-between">
          Create new project
          <button @click="showNewProject=false" class="text-gray-400 hover:text-gray-600 text-lg leading-none">&times;</button>
        </div>
        <input v-model="newName" placeholder="Project name" class="mb-3" @keyup.enter="createProject" />
        <select v-model="newVis" class="mb-3">
          <option value="private">Private</option>
          <option value="public">Public</option>
          <option value="internal">Internal</option>
        </select>
        <div class="flex gap-2">
          <button @click="createProject" :disabled="!newName" class="btn btn-confirm">Create project</button>
          <button @click="showNewProject=false" class="btn btn-default">Cancel</button>
        </div>
        <p v-if="newError" class="text-red-500 text-xs mt-2">{{ newError }}</p>
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

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const q = ref("");
const repos = ref<any[]>([]);
const showNewProject = ref(false);
const newName = ref("");
const newVis = ref("private");
const newError = ref("");

function doSearch() {
  if (q.value.trim()) router.push(`/search?q=${encodeURIComponent(q.value.trim())}`);
}

async function createProject() {
  if (!newName.value) return;
  try {
    const r = await api.post("/projects/", { name: newName.value, visibility: newVis.value });
    showNewProject.value = false;
    newName.value = "";
    router.push(`/${r.path}`);
  } catch (e: any) { newError.value = e.message; }
}

onMounted(async () => {
  if (auth.token) {
    auth.fetchMe();
    try { repos.value = (await api.get("/projects/"))?.slice(0, 15) || []; } catch {}
  }
});
</script>
