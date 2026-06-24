<template>
  <AppLayout>
    <div class="max-w-4xl mx-auto">
      <h1 class="text-2xl font-bold mb-4">Welcome to MyGit</h1>
      <p class="text-gray-600 dark:text-slate-400 mb-6">Self-hosted Git platform</p>
      <div v-if="auth.user" class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <RouterLink :to="`/${auth.user.username}`" class="p-4 border rounded-lg hover:shadow">
          <h3 class="font-semibold">My Profile</h3>
          <p class="text-sm text-gray-500">View your repositories and activity</p>
        </RouterLink>
        <RouterLink to="/groups" class="p-4 border rounded-lg hover:shadow">
          <h3 class="font-semibold">Groups</h3>
          <p class="text-sm text-gray-500">Browse organizations and teams</p>
        </RouterLink>
      </div>
      <div v-else class="flex gap-4">
        <RouterLink to="/auth/login" class="px-4 py-2 bg-blue-600 text-white rounded">Login</RouterLink>
        <RouterLink to="/auth/register" class="px-4 py-2 border rounded">Register</RouterLink>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from "../components/AppLayout.vue";
import { useAuthStore } from "../stores/auth";
const auth = useAuthStore();
</script>
