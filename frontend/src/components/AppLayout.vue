<template>
  <div class="flex h-screen overflow-hidden">
    <!-- Sidebar -->
    <aside class="w-[220px] bg-white dark:bg-slate-900 border-r border-gray-200 dark:border-slate-700 flex flex-col shrink-0">
      <div class="px-4 py-3 border-b border-gray-200 dark:border-slate-700">
        <RouterLink to="/" class="text-lg font-bold text-blue-600 no-underline">MyGit</RouterLink>
      </div>
      <nav class="flex-1 overflow-y-auto px-3 py-2">
        <RouterLink to="/" class="flex items-center gap-2 px-3 py-2 rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-800 no-underline mb-0.5">
          🏠 Home
        </RouterLink>
        <RouterLink v-if="auth.user" :to="`/${auth.user.username}`" class="flex items-center gap-2 px-3 py-2 rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-800 no-underline mb-0.5">
          👤 Profile
        </RouterLink>
        <RouterLink to="/groups" class="flex items-center gap-2 px-3 py-2 rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-800 no-underline mb-0.5">
          👥 Groups
        </RouterLink>
        <RouterLink to="/search" class="flex items-center gap-2 px-3 py-2 rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-800 no-underline">
          🔍 Search
        </RouterLink>

        <div v-if="auth.user && repos.length" class="mt-4">
          <div class="px-3 py-1 text-xs font-semibold text-gray-400 uppercase tracking-wide">Projects</div>
          <RouterLink v-for="r in repos" :key="r.id" :to="`/${r.path}`" class="flex items-center gap-2 px-3 py-1.5 rounded text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-800 no-underline truncate">
            📁 {{ r.name }}
          </RouterLink>
        </div>
      </nav>
      <div class="px-4 py-3 border-t border-gray-200 dark:border-slate-700 text-xs">
        <template v-if="auth.user">
          <div class="text-gray-500 mb-1">{{ auth.user.username }}</div>
          <button @click="auth.logout()" class="text-red-500 hover:text-red-600">Sign out</button>
        </template>
        <template v-else>
          <RouterLink to="/auth/login" class="text-blue-600 mr-3">Sign in</RouterLink>
          <RouterLink to="/auth/register" class="text-blue-600">Register</RouterLink>
        </template>
      </div>
    </aside>

    <!-- Main content -->
    <div class="flex-1 flex flex-col overflow-hidden">
      <header class="bg-white dark:bg-slate-900 border-b border-gray-200 dark:border-slate-700 px-6 py-3 flex items-center gap-4 shrink-0">
        <form @submit.prevent="doSearch" class="flex-1 max-w-md">
          <input v-model="q" placeholder="Search projects, issues, MRs..." class="w-full py-1.5 px-3 text-sm" />
        </form>
        <template v-if="auth.user">
          <RouterLink to="/notifications" class="text-sm text-gray-500 hover:text-gray-700">🔔</RouterLink>
        </template>
      </header>
      <main class="flex-1 overflow-y-auto p-6">
        <router-view />
      </main>
    </div>
    <Toast />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { api } from "../api/client";
import Toast from "./Toast.vue";

const router = useRouter();
const auth = useAuthStore();
const q = ref("");
const repos = ref<any[]>([]);

function doSearch() {
  if (q.value.trim()) router.push(`/search?q=${encodeURIComponent(q.value.trim())}`);
}

onMounted(async () => {
  if (auth.token) {
    auth.fetchMe();
    try { repos.value = (await api.get("/projects/"))?.slice(0, 15) || []; } catch {}
  }
});
</script>
