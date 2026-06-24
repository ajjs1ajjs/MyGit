<template>
  <AppLayout>
    <div class="max-w-md mx-auto mt-12">
      <h1 class="text-2xl font-bold mb-6">Login</h1>
      <form @submit.prevent="handleLogin" class="flex flex-col gap-4">
        <input v-model="email" type="email" placeholder="Email" required class="px-3 py-2 border rounded" />
        <input v-model="password" type="password" placeholder="Password" required class="px-3 py-2 border rounded" />
        <p v-if="error" class="text-red-600 text-sm">{{ error }}</p>
        <button type="submit" class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700">Login</button>
      </form>
      <p class="mt-4 text-sm text-gray-500">
        No account? <RouterLink to="/auth/register" class="text-blue-600">Register</RouterLink>
      </p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import AppLayout from "../components/AppLayout.vue";

const router = useRouter();
const auth = useAuthStore();
const email = ref("");
const password = ref("");
const error = ref("");

async function handleLogin() {
  try {
    await auth.login(email.value, password.value);
    router.push("/");
  } catch (e: any) {
    error.value = e.message;
  }
}
</script>
