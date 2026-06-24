<template>
  <div class="flex h-screen overflow-hidden">
    <aside class="sidebar">
      <RouterLink to="/" class="logo !text-white !no-underline">MyGit</RouterLink>
      <nav class="flex-1 overflow-y-auto">
        <RouterLink to="/" class="nav-item">
          <svg class="icon" viewBox="0 0 24 24"><path d="M3 9l9-7 9 7v11a2 2 0 01-2 2H5a2 2 0 01-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>
          Home
        </RouterLink>
        <RouterLink v-if="auth.user" :to="`/${auth.user.username}`" class="nav-item">
          <svg class="icon" viewBox="0 0 24 24"><path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
          Profile
        </RouterLink>
        <RouterLink to="/groups" class="nav-item">
          <svg class="icon" viewBox="0 0 24 24"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"/></svg>
          Groups
        </RouterLink>
        <RouterLink to="/search" class="nav-item">
          <svg class="icon" viewBox="0 0 24 24"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>
          Search
        </RouterLink>
        <template v-if="repos.length">
          <div class="section">Projects</div>
          <RouterLink v-for="r in repos" :key="r.id" :to="`/${r.path}`" class="nav-item truncate">
            <svg class="icon" viewBox="0 0 24 24"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
            {{ r.name }}
          </RouterLink>
        </template>
      </nav>
      <div class="footer">
        <template v-if="auth.user">
          <div class="text-white/80 text-sm">{{ auth.user.username }}</div>
          <button @click="auth.logout()" class="text-white/40 hover:text-white/80 mt-1 text-xs">Sign out</button>
        </template>
        <template v-else>
          <RouterLink to="/auth/login" class="block text-blue-300 text-sm">Sign in</RouterLink>
          <RouterLink to="/auth/register" class="block text-blue-300 text-xs mt-1">Register</RouterLink>
        </template>
      </div>
    </aside>
    <div class="flex-1 flex flex-col overflow-hidden">
      <header class="topbar">
        <form @submit.prevent="search" class="flex-1 max-w-md">
          <input v-model="q" placeholder="Search..." />
        </form>
        <button v-if="auth.user" @click="showNew=true" class="btn btn-primary btn-sm">+ New project</button>
      </header>
      <main class="flex-1 overflow-y-auto p-6">
        <router-view />
      </main>
    </div>
    <div v-if="showNew" class="modal-overlay" @click.self="showNew=false">
      <div class="modal">
        <div class="card-header flex items-center justify-between">New project<button @click="showNew=false" class="text-gray-400 text-lg leading-none">&times;</button></div>
        <div class="card-body"><input v-model="newName" placeholder="Project name" class="mb-3" @keyup.enter="create" /><select v-model="newVis" class="mb-3"><option value="private">Private</option><option value="public">Public</option></select><div class="flex gap-2"><button @click="create" :disabled="!newName" class="btn btn-primary btn-sm">Create</button><button @click="showNew=false" class="btn btn-outline btn-sm">Cancel</button></div></div>
      </div>
    </div>
    <Toast />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"; import { useRouter } from "vue-router"; import { useAuthStore } from "../stores/auth"; import { api } from "../api/client"; import Toast from "./Toast.vue";
const router = useRouter(); const auth = useAuthStore(); const q = ref(""); const repos = ref<any[]>([]); const showNew = ref(false); const newName = ref(""); const newVis = ref("private");
function search(){ if(q.value.trim()) router.push(`/search?q=${encodeURIComponent(q.value.trim())}`) }
async function create(){ if(!newName.value)return; try{ const r=await api.post("/projects/",{name:newName.value,visibility:newVis.value}); showNew.value=false; newName.value=''; router.push(`/${r.path}`); }catch{} }
onMounted(async ()=>{ if(auth.token){ auth.fetchMe(); try{ repos.value=((await api.get("/projects/"))||[]).slice(0,15) }catch{} } });
</script>
