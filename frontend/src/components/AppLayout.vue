<template>
  <div class="flex h-screen overflow-hidden">
    <aside class="sidebar">
      <RouterLink to="/" class="logo !no-underline">MyGit</RouterLink>
      <nav class="flex-1 overflow-y-auto py-1">
        <RouterLink to="/" class="nav-item" active-class="active">
          <svg class="w-4 h-4 opacity-60" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 9l9-7 9 7v11a2 2 0 01-2 2H5a2 2 0 01-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>
          Home
        </RouterLink>
        <RouterLink v-if="auth.user" :to="`/${auth.user.username}`" class="nav-item" active-class="active">
          <svg class="w-4 h-4 opacity-60" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
          Profile
        </RouterLink>
        <RouterLink to="/groups" class="nav-item" active-class="active">
          <svg class="w-4 h-4 opacity-60" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"/></svg>
          Groups
        </RouterLink>
        <RouterLink to="/search" class="nav-item" active-class="active">
          <svg class="w-4 h-4 opacity-60" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>
          Search
        </RouterLink>
        <RouterLink v-if="auth.user?.is_superuser" to="/manage/users" class="nav-item" active-class="active">
          <svg class="w-4 h-4 opacity-60" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/></svg>
          Users
        </RouterLink>
        <RouterLink v-if="auth.user?.is_superuser" to="/manage/system" class="nav-item" active-class="active">
          <svg class="w-4 h-4 opacity-60" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 00.34 1.88l.06.06a2 2 0 01-2.83 2.83l-.06-.06A1.7 1.7 0 0015 19.4a1.7 1.7 0 00-1 .6 1.7 1.7 0 00-.4 1.1V21a2 2 0 01-4 0v-.09A1.7 1.7 0 008.6 19.4a1.7 1.7 0 00-1.88.34l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.7 1.7 0 004.6 15a1.7 1.7 0 00-.6-1 1.7 1.7 0 00-1.1-.4H3a2 2 0 010-4h.09A1.7 1.7 0 004.6 8.6a1.7 1.7 0 00-.34-1.88l-.06-.06a2 2 0 012.83-2.83l.06.06A1.7 1.7 0 009 4.6a1.7 1.7 0 001-.6 1.7 1.7 0 00.4-1.1V3a2 2 0 014 0v.09A1.7 1.7 0 0015.4 4.6a1.7 1.7 0 001.88-.34l.06-.06a2 2 0 012.83 2.83l-.06.06A1.7 1.7 0 0019.4 9c.36.22.72.44 1 .6.33.18.7.28 1.1.28H21a2 2 0 010 4h-.09A1.7 1.7 0 0019.4 15z"/></svg>
          System
        </RouterLink>
        <div class="section">Projects</div>
        <template v-if="repos.length">
          <RouterLink v-for="r in repos" :key="r.id" :to="`/${r.path}`" class="nav-item truncate" active-class="active">
            <svg class="w-4 h-4 opacity-60 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
            {{ r.name }}
          </RouterLink>
        </template>
        <RouterLink v-else-if="auth.user" to="/" class="nav-item text-[#525252]">
          <svg class="w-4 h-4 opacity-40 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          New project...
        </RouterLink>
      </nav>
      <div class="footer">
        <template v-if="auth.user">
          <div class="text-xs text-[#737373]">{{ auth.user.username }}</div>
          <button @click="auth.logout()" class="text-xs text-[#525252] hover:text-[#a3a3a3] mt-0.5 transition-colors">Sign out</button>
        </template>
        <template v-else>
          <RouterLink to="/auth/login" class="block text-blue-400 text-xs hover:text-blue-300">Sign in</RouterLink>
          <RouterLink to="/auth/register" class="block text-blue-400 text-xs mt-0.5 hover:text-blue-300">Register</RouterLink>
        </template>
      </div>
    </aside>
    <div class="flex-1 flex flex-col overflow-hidden">
      <header class="topbar">
        <form @submit.prevent="search" class="flex-1"><input v-model="q" placeholder="Search..." /></form>
        <RouterLink v-if="auth.user" to="/projects/new" class="btn btn-accent btn-sm">+ New project</RouterLink>
      </header>
      <main class="flex-1 overflow-y-auto p-6">
        <router-view />
      </main>
    </div>
    <Toast />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"; import { useRouter } from "vue-router"; import { useAuthStore } from "../stores/auth"; import { api } from "../api/client"; import Toast from "./Toast.vue";
const router = useRouter(); const auth = useAuthStore(); const q = ref(""); const repos = ref<any[]>([]);
function search(){ if(q.value.trim()) router.push(`/search?q=${encodeURIComponent(q.value.trim())}`) }
onMounted(async ()=>{ await auth.fetchMe(); if(auth.user){ try{ repos.value=((await api.get("/projects/"))||[]).slice(0,20) }catch{} } });
</script>
