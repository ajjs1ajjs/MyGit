<template>
  <div class="max-w-md mx-auto">
    <h1 class="text-xl font-bold mb-6">Sign in to MyGit</h1>
    <form @submit.prevent="handle" class="flex flex-col gap-4">
      <div>
        <label class="text-xs text-gray-500 mb-1 block">Email</label>
        <input v-model="email" type="email" required class="w-full" />
      </div>
      <div>
        <label class="text-xs text-gray-500 mb-1 block">Password</label>
        <input v-model="password" type="password" required class="w-full" />
      </div>
      <p v-if="error" class="text-red-500 text-xs">{{ error }}</p>
      <button type="submit" class="btn btn-primary w-full justify-center">Sign in</button>
    </form>
    <p class="mt-4 text-sm text-gray-500">
      No account? <RouterLink to="/auth/register">Create one</RouterLink>
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
const router = useRouter(); const auth = useAuthStore();
const email = ref(""); const password = ref(""); const error = ref("");
async function handle() {
  try { await auth.login(email.value, password.value); router.push("/"); }
  catch (e: any) { error.value = e.message; }
}
</script>
