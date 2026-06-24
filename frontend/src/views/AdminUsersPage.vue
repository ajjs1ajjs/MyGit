<template>
  <div class="max-w-5xl">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-lg font-semibold">Users</h1>
      <button @click="openInvite()" class="btn btn-accent btn-sm">Invite user</button>
    </div>

    <div class="card mb-4">
      <div class="card-header">
        <input v-model="search" placeholder="Search users..." class="max-w-xs !py-1.5 !text-xs" />
        <span class="text-xs text-[#a3a3a3]">{{ filtered.length }} user{{ filtered.length !== 1 ? 's' : '' }}</span>
      </div>
      <table v-if="filtered.length">
        <thead><tr><th>User</th><th>Email</th><th>Role</th><th>Active</th><th>Joined</th><th></th></tr></thead>
        <tbody>
          <tr v-for="u in filtered" :key="u.id">
            <td><div class="flex items-center gap-2"><div class="w-7 h-7 rounded-full bg-[#2563eb] text-white flex items-center justify-center text-xs font-bold shrink-0">{{ u.username[0].toUpperCase() }}</div><span class="font-medium">{{ u.username }}</span></div></td>
            <td class="text-[#737373] text-sm">{{ u.email }}</td>
            <td><span class="badge" :class="u.is_superuser?'badge-purple':'badge-gray'">{{ u.is_superuser ? 'Admin' : 'User' }}</span></td>
            <td><span class="badge" :class="u.is_active?'badge-green':'badge-red'">{{ u.is_active ? 'Yes' : 'No' }}</span></td>
            <td class="text-xs text-[#a3a3a3]">{{ fmt(u.date_joined) }}</td>
            <td>
              <div class="flex gap-1">
                <button @click="openEdit(u)" class="btn btn-ghost btn-sm">Edit</button>
                <button @click="toggleAdmin(u)" class="btn btn-ghost btn-sm">{{ u.is_superuser ? 'Demote' : 'Admin' }}</button>
                <button @click="toggleActive(u)" class="btn btn-ghost btn-sm">{{ u.is_active ? 'Block' : 'Activate' }}</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="empty-state !py-8"><div class="icon">&#128269;</div><h3>No users found</h3></div>
    </div>

    <!-- Edit user modal -->
    <div v-if="editUser" class="modal-overlay" @click.self="editUser=null">
      <div class="modal">
        <div class="card-header">Edit {{ editUser.username }} <button @click="editUser=null" class="text-[#a3a3a3] hover:text-[#737373] text-lg leading-none">&times;</button></div>
        <div class="card-body space-y-3">
          <div><label class="text-xs font-medium mb-1 block">Username</label><input v-model="editForm.username" /></div>
          <div><label class="text-xs font-medium mb-1 block">Email</label><input v-model="editForm.email" type="email" /></div>
          <div><label class="text-xs font-medium mb-1 block">Full name</label><input v-model="editForm.full_name" /></div>
          <div><label class="text-xs font-medium mb-1 block">New password (leave empty to keep)</label><input v-model="editForm.password" type="password" /></div>
          <div class="flex items-center gap-2"><input type="checkbox" v-model="editForm.is_superuser" class="w-auto" /><label class="text-xs">Admin</label></div>
          <div class="flex items-center gap-2"><input type="checkbox" v-model="editForm.is_active" class="w-auto" /><label class="text-xs">Active</label></div>
          <p v-if="editErr" class="text-xs text-[#dc2626]">{{ editErr }}</p>
          <div class="flex gap-2 pt-1"><button @click="saveEdit" class="btn btn-accent">Save</button><button @click="editUser=null" class="btn btn-ghost">Cancel</button></div>
        </div>
      </div>
    </div>

    <!-- Invite modal -->
    <div v-if="showInvite" class="modal-overlay" @click.self="showInvite=false">
      <div class="modal"><div class="card-header">Invite user <button @click="showInvite=false" class="text-[#a3a3a3] hover:text-[#737373] text-lg leading-none">&times;</button></div>
        <div class="card-body space-y-3">
          <div><label class="text-xs font-medium mb-1 block">Username</label><input v-model="invUsername" /></div>
          <div><label class="text-xs font-medium mb-1 block">Password</label><input v-model="invPass" type="password" /></div>
          <div class="flex items-center gap-2"><input type="checkbox" v-model="invAdmin" class="w-auto" /><label class="text-xs">Admin</label></div>
          <p v-if="invErr" class="text-xs text-[#dc2626]">{{ invErr }}</p>
          <div class="flex gap-2 pt-1"><button @click="invite" :disabled="!invUsername||!invPass" class="btn btn-accent">Create</button><button @click="showInvite=false" class="btn btn-ghost">Cancel</button></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from "vue"; import { api } from "../api/client";
const users = ref<any[]>([]); const search = ref(""); const showInvite = ref(false);
const invUsername = ref(""); const invPass = ref(""); const invAdmin = ref(false); const invErr = ref("");
const editUser = ref<any>(null); const editErr = ref("");
const editForm = reactive({ username: "", email: "", full_name: "", password: "", is_superuser: false, is_active: true });

const filtered = computed(() => {
  if (!search.value) return users.value;
  const q = search.value.toLowerCase();
  return users.value.filter((u:any) => u.username.toLowerCase().includes(q) || u.email.toLowerCase().includes(q));
});

function openEdit(u: any) {
  editUser.value = u;
  editForm.username = u.username;
  editForm.email = u.email;
  editForm.full_name = u.full_name || "";
  editForm.password = "";
  editForm.is_superuser = u.is_superuser;
  editForm.is_active = u.is_active;
}
function openInvite() { invUsername.value=''; invPass.value=''; invAdmin.value=false; invErr.value=''; showInvite.value=true; }

async function saveEdit() {
  try {
    const data: any = { username: editForm.username, email: editForm.email, full_name: editForm.full_name, is_superuser: editForm.is_superuser, is_active: editForm.is_active };
    if (editForm.password) data.password = editForm.password;
    await api.patch(`/users/${editUser.value.username}/`, data);
    Object.assign(editUser.value, data);
    editUser.value = null;
  } catch (e: any) { editErr.value = e.message; }
}

async function toggleAdmin(u: any) { try { await api.patch(`/users/${u.username}/`,{is_superuser:!u.is_superuser}); u.is_superuser=!u.is_superuser; } catch {} }
async function toggleActive(u: any) { try { await api.patch(`/users/${u.username}/`,{is_active:!u.is_active}); u.is_active=!u.is_active; } catch {} }
async function invite() { try { await api.post("/auth/register/",{username:invUsername.value,password:invPass.value}); showInvite.value=false; users.value=(await api.get("/users/"))||[]; }catch(e:any){invErr.value=e.message} }

function fmt(d: string) { return d ? new Date(d).toLocaleDateString() : ''; }
onMounted(async () => { try { users.value = (await api.get("/users/")) || []; } catch {} });
</script>
