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
          <div class="text-xs text-[#737373]">{{ auth.user.email }}</div>
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
        <a v-if="!auth.user" href="/django-admin/" target="_blank" class="btn btn-ghost btn-sm">Admin panel</a>
        <button v-if="auth.user" @click="showNew=true" class="btn btn-accent btn-sm">+ New project</button>
      </header>
      <main class="flex-1 overflow-y-auto p-6">
        <router-view />
      </main>
    </div>
    <div v-if="showNew" class="modal-overlay" @click.self="showNew=false">
      <div class="modal">
        <div class="card-header">Create new project <button @click="showNew=false" class="text-[#a3a3a3] hover:text-[#737373] text-lg leading-none">&times;</button></div>
        <div class="card-body space-y-3">
          <div><label class="text-xs font-medium mb-1 block">Project name</label><input v-model="newName" @keyup.enter="create" /></div>
          <div class="flex gap-2"><button @click="newVis='private'" :class="newVis==='private'?'btn-accent':'btn-ghost'" class="btn btn-sm flex-1">Private</button><button @click="newVis='public'" :class="newVis==='public'?'btn-accent':'btn-ghost'" class="btn btn-sm flex-1">Public</button></div>
          <div class="flex gap-2 pt-1"><button @click="create" :disabled="!newName" class="btn btn-accent">Create</button><button @click="showNew=false" class="btn btn-ghost">Cancel</button></div>
        </div>
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
onMounted(async ()=>{ if(auth.token){ auth.fetchMe(); try{ repos.value=((await api.get("/projects/"))||[]).slice(0,20) }catch{} } });
</script>
