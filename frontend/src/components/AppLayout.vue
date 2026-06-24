<template>
  <div class="min-h-screen flex flex-col">
    <nav class="bg-white dark:bg-slate-800 border-b border-gray-200 dark:border-slate-700 px-4 py-3 flex items-center justify-between">
      <RouterLink to="/" class="text-lg font-bold text-blue-600">MyGit</RouterLink>
      <div class="flex items-center gap-4">
        <SearchBar />
        <template v-if="auth.user">
          <RouterLink :to="`/${auth.user.username}`" class="text-sm hover:text-blue-600">{{ auth.user.username }}</RouterLink>
          <button @click="auth.logout()" class="text-sm text-gray-500 hover:text-red-600">Logout</button>
        </template>
        <template v-else>
          <RouterLink to="/auth/login" class="text-sm hover:text-blue-600">Login</RouterLink>
          <RouterLink to="/auth/register" class="text-sm hover:text-blue-600">Register</RouterLink>
        </template>
      </div>
    </nav>
    <main class="flex-1 p-6">
      <router-view />
    </main>
    <Toast />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue";
import { useAuthStore } from "../stores/auth";
import SearchBar from "./SearchBar.vue";
import Toast from "./Toast.vue";

const auth = useAuthStore();
onMounted(() => {
  if (auth.token) auth.fetchMe();
});
</script>
