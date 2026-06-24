<template>
  <div class="max-w-5xl">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-lg font-semibold">Users</h1>
      <button @click="showInvite=true" class="btn btn-accent btn-sm">Invite user</button>
    </div>

    <div class="card mb-4">
      <div class="card-header">
        <div class="flex items-center gap-2">
          <input v-model="search" placeholder="Search by username or email..." class="max-w-xs !py-1.5 !text-xs" />
          <span class="text-xs text-[#a3a3a3]">{{ filtered.length }} user{{ filtered.length !== 1 ? 's' : '' }}</span>
        </div>
      </div>
      <table v-if="filtered.length">
        <thead><tr><th>User</th><th>Email</th><th>Role</th><th>Status</th><th>Joined</th><th></th></tr></thead>
        <tbody>
          <tr v-for="u in filtered" :key="u.id">
            <td>
              <div class="flex items-center gap-2">
                <div class="w-7 h-7 rounded-full bg-[#2563eb] text-white flex items-center justify-center text-xs font-bold shrink-0">{{ u.username[0].toUpperCase() }}</div>
                <span class="font-medium">{{ u.username }}</span>
              </div>
            </td>
            <td class="text-[#737373] text-sm">{{ u.email }}</td>
            <td>
              <span class="badge" :class="u.is_superuser?'badge-purple':'badge-gray'">{{ u.is_superuser ? 'Admin' : 'User' }}</span>
            </td>
            <td>
              <span class="badge" :class="u.is_active?'badge-green':'badge-red'">{{ u.is_active ? 'Active' : 'Inactive' }}</span>
            </td>
            <td class="text-xs text-[#a3a3a3]">{{ fmt(u.date_joined) }}</td>
            <td>
              <div class="flex gap-1">
                <button v-if="!u.is_superuser" @click="toggleAdmin(u)" class="btn btn-ghost btn-sm">{{ u.is_superuser ? 'Demote' : 'Make admin' }}</button>
                <button @click="toggleActive(u)" class="btn btn-ghost btn-sm">{{ u.is_active ? 'Deactivate' : 'Activate' }}</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="empty-state !py-8"><div class="icon">&#128269;</div><h3>No users found</h3></div>
    </div>

    <div class="grid grid-cols-2 gap-4 text-sm">
      <div class="card">
        <div class="card-body"><div class="font-semibold mb-1">Quick stats</div>
          <div class="flex gap-4 text-xs text-[#737373]">
            <span>{{ users.filter((u:any)=>u.is_active).length }} active</span>
            <span>{{ users.filter((u:any)=>u.is_superuser).length }} admins</span>
            <span>{{ users.length }} total</span>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showInvite" class="modal-overlay" @click.self="showInvite=false">
      <div class="modal">
        <div class="card-header">Invite user <button @click="showInvite=false" class="text-[#a3a3a3] hover:text-[#737373] text-lg leading-none">&times;</button></div>
        <div class="card-body space-y-3">
          <div><label class="text-xs font-medium mb-1 block">Username</label><input v-model="invUsername" /></div>
          <div><label class="text-xs font-medium mb-1 block">Email</label><input v-model="invEmail" type="email" /></div>
          <div><label class="text-xs font-medium mb-1 block">Password</label><input v-model="invPass" type="password" /></div>
          <div class="flex items-center gap-2"><input type="checkbox" v-model="invAdmin" class="w-auto" /><label class="text-xs">Admin privileges</label></div>
          <p v-if="invErr" class="text-xs text-[#dc2626]">{{ invErr }}</p>
          <div class="flex gap-2 pt-1"><button @click="invite" :disabled="!invUsername||!invEmail||!invPass" class="btn btn-accent">Create user</button><button @click="showInvite=false" class="btn btn-ghost">Cancel</button></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue"; import { api } from "../api/client";
const users = ref<any[]>([]); const search = ref(""); const showInvite = ref(false);
const invUsername = ref(""); const invEmail = ref(""); const invPass = ref(""); const invAdmin = ref(false); const invErr = ref("");
const filtered = computed(() => {
  if (!search.value) return users.value;
  const q = search.value.toLowerCase();
  return users.value.filter((u:any) => u.username.toLowerCase().includes(q) || u.email.toLowerCase().includes(q));
});
async function toggleAdmin(u: any) { try { await api.patch(`/users/${u.username}/`,{is_superuser:!u.is_superuser}); u.is_superuser=!u.is_superuser; } catch {} }
async function toggleActive(u: any) { try { await api.patch(`/users/${u.username}/`,{is_active:!u.is_active}); u.is_active=!u.is_active; } catch {} }
async function invite() { try { await api.post("/auth/register/",{username:invUsername.value,email:invEmail.value,password:invPass.value}); showInvite.value=false; invUsername.value=''; invEmail.value=''; invPass.value=''; const all=await api.get("/users/")||[]; users.value=all; } catch(e:any) { invErr.value=e.message } }
function fmt(d: string) { return d ? new Date(d).toLocaleDateString() : ''; }
onMounted(async () => { try { users.value = (await api.get("/users/")) || []; } catch {} });
</script>
