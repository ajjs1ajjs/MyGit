<template>
  <div class="max-w-4xl">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-lg font-semibold">Groups</h1>
      <button @click="showNew=true" class="btn btn-accent btn-sm">New group</button>
    </div>

    <div v-if="groups.length" class="space-y-3">
      <RouterLink v-for="g in groups" :key="g.id" :to="`/groups/${g.id}`" class="card flex items-center gap-3 !no-underline group">
        <div class="card-body flex items-center gap-3 w-full">
          <svg class="w-5 h-5 text-[#a3a3a3] shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"/></svg>
          <div class="flex-1 min-w-0">
            <div class="font-medium text-sm group-hover:text-[#2563eb]">{{g.path}}</div>
            <div class="text-xs text-[#a3a3a3] mt-0.5">{{g.description||g.name}} &middot; {{g.member_count||0}} members</div>
          </div>
        </div>
      </RouterLink>
    </div>
    <div v-else class="empty-state"><div class="icon">&#128101;</div><h3>No groups yet</h3><p>Create a group to collaborate with your team.</p></div>

    <div v-if="showNew" class="modal-overlay" @click.self="showNew=false">
      <div class="modal"><div class="card-header">Create group <button @click="showNew=false" class="text-[#a3a3a3] hover:text-[#737373] text-lg leading-none">&times;</button></div>
        <div class="card-body space-y-3">
          <div><label class="text-xs font-medium mb-1 block">Group name</label><input v-model="name" placeholder="my-team" @keyup.enter="create" /></div>
          <div><label class="text-xs font-medium mb-1 block">Description</label><input v-model="desc" placeholder="Optional" /></div>
          <p v-if="err" class="text-xs text-[#dc2626]">{{err}}</p>
          <div class="flex gap-2 pt-1"><button @click="create" :disabled="!name" class="btn btn-accent">Create group</button><button @click="showNew=false" class="btn btn-ghost">Cancel</button></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"; import { useRouter } from "vue-router"; import { api } from "../api/client";
const router = useRouter(); const groups = ref<any[]>([]); const showNew = ref(false); const name = ref(""); const desc = ref(""); const err = ref("");
async function create(){ if(!name.value)return; try{ const g=await api.post("/groups/",{name:name.value,path:name.value.toLowerCase().replace(/\s+/g,'-'),description:desc.value}); showNew=false; name.value=''; desc.value=''; router.push(`/groups/${g.id}`); }catch(e:any){ err.value=e.message } }
onMounted(async ()=>{ try{ groups.value=(await api.get("/groups/"))||[] }catch{} });
</script>
