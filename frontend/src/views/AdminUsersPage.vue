<template>
  <div class="max-w-5xl">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-lg font-semibold">Users</h1>
      <RouterLink to="/auth/register" class="btn btn-accent btn-sm">Invite user</RouterLink>
    </div>
    <div v-if="users.length" class="card">
      <table>
        <thead><tr><th>Username</th><th>Email</th><th>Role</th><th>Joined</th><th></th></tr></thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td><div class="font-medium">{{u.username}}</div></td>
            <td class="text-[#737373]">{{u.email}}</td>
            <td>
              <span class="badge" :class="u.is_superuser?'badge-purple':'badge-gray'">{{u.is_superuser?'Admin':'User'}}</span>
            </td>
            <td class="text-xs text-[#a3a3a3]">{{fmt(u.date_joined)}}</td>
            <td>
              <button v-if="!u.is_superuser || u.id !== auth.user?.id" @click="toggleAdmin(u)" class="btn btn-ghost btn-sm">
                {{u.is_superuser?'Demote':'Make admin'}}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="empty-state"><div class="icon">&#128100;</div><h3>No users</h3></div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"; import { api } from "../api/client"; import { useAuthStore } from "../stores/auth";
const auth = useAuthStore(); const users = ref<any[]>([]);
async function toggleAdmin(u: any) {
  try { await api.patch(`/users/${u.username}/`, { is_superuser: !u.is_superuser }); u.is_superuser = !u.is_superuser; } catch {}
}
function fmt(d: string) { return d ? new Date(d).toLocaleDateString() : ''; }
onMounted(async ()=>{ try{ users.value = (await api.get("/users/")) || [] }catch{} });
</script>
