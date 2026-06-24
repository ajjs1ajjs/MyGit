<template>
  <div class="max-w-4xl">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-lg font-semibold">Projects</h1>
      <RouterLink v-if="auth.user" to="/projects/new" class="btn btn-accent btn-sm">New project</RouterLink>
    </div>

    <div v-if="!auth.user" class="card mb-6">
      <div class="card-body flex items-center justify-between">
        <div><h3 class="font-semibold mb-1">Welcome to MyGit</h3><p class="text-sm text-[#737373]">Self-hosted Git platform for your team.</p></div>
        <div class="flex gap-2"><RouterLink to="/auth/login" class="btn btn-accent btn-sm">Sign in</RouterLink><RouterLink to="/auth/register" class="btn btn-ghost btn-sm">Register</RouterLink></div>
      </div>
    </div>

    <div v-if="auth.user" class="navtabs mb-5">
      <button @click="filter='all'" :class="{active:filter==='all'}" class="border-0 bg-transparent cursor-pointer font-inherit">All projects</button>
      <button @click="filter='yours'" :class="{active:filter==='yours'}" class="border-0 bg-transparent cursor-pointer font-inherit">Your projects</button>
    </div>

    <div v-if="filtered.length" class="space-y-3">
      <RouterLink v-for="r in filtered" :key="r.id" :to="`/${r.path}`" class="card flex items-center gap-3 !no-underline group">
        <div class="card-body flex items-center gap-3 w-full">
          <svg class="w-5 h-5 text-[#a3a3a3] shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
          <div class="flex-1 min-w-0">
            <div class="font-medium text-sm group-hover:text-[#2563eb]">{{r.name}}</div>
            <div class="text-xs text-[#a3a3a3] mt-0.5">{{r.path}} &middot; {{fmt(r.updated_at)}}</div>
          </div>
          <span class="badge" :class="r.visibility==='public'?'badge-blue':'badge-gray'">{{r.visibility}}</span>
        </div>
      </RouterLink>
    </div>
    <div v-else-if="auth.user" class="empty-state"><div class="icon">&#128193;</div><h3>No projects yet</h3><p>Create your first project to get started.</p></div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue"; import { useRouter } from "vue-router"; import { useAuthStore } from "../stores/auth"; import { api } from "../api/client";
const router = useRouter(); const auth = useAuthStore();
const repos = ref<any[]>([]); const filter = ref("all");
const filtered = computed(() => filter.value==='yours'&&auth.user ? repos.value.filter((r:any)=>r.path?.startsWith(auth.user!.username+'/')) : repos.value);
function fmt(d:string){ return d?new Date(d).toLocaleDateString('en-US',{month:'short',day:'numeric'}):'' }
onMounted(async ()=>{ try{ repos.value=(await api.get("/projects/"))||[] }catch{} });
</script>

