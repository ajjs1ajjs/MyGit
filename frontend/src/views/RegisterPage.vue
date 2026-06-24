<template>
  <div class="max-w-md mx-auto">
    <h1 class="text-xl font-bold mb-6">Create account</h1>
    <form @submit.prevent="handle" class="flex flex-col gap-4">
      <div><label for="signup-username" class="text-xs text-gray-500 mb-1 block">Username</label><input id="signup-username" name="username" v-model="username" required autocomplete="username" class="w-full" /></div>
      <div><label for="new-password" class="text-xs text-gray-500 mb-1 block">Password</label><input id="new-password" name="new-password" v-model="password" type="password" required minlength="8" autocomplete="new-password" class="w-full" /></div>
      <p v-if="error" class="text-red-500 text-xs">{{ error }}</p>
      <button type="submit" class="btn btn-primary w-full justify-center">Register</button>
    </form>
    <p class="mt-4 text-sm text-gray-500">Have an account? <RouterLink to="/auth/login">Sign in</RouterLink></p>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"; import { useRouter } from "vue-router"; import { useAuthStore } from "../stores/auth";
const router = useRouter(); const auth = useAuthStore();
const username = ref(""); const password = ref(""); const error = ref("");
async function handle() {
  try { await auth.register(username.value, password.value); router.push("/"); }
  catch (e: any) { error.value = e.message; }
}
</script>
