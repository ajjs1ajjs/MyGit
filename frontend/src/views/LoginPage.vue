<template>
  <div class="min-h-screen bg-[#f8f9fa] flex items-center justify-center p-4">
    <div class="w-full max-w-sm">
      <div class="text-center mb-8">
        <div class="text-2xl font-bold text-[#212529]">MyGit</div>
        <p class="text-sm text-[#6c757d] mt-1">Sign in to continue</p>
      </div>
      <div class="bg-white dark:bg-[#16213e] border border-[#dee2e6] dark:border-[#2a2a4a] rounded-xl p-6 shadow-sm">
        <form @submit.prevent="go" class="space-y-4">
          <div><label class="text-xs font-medium text-[#212529] dark:text-white block mb-1.5">Email address</label><input v-model="email" type="text" required autocomplete="username" /></div>
          <div><label class="text-xs font-medium text-[#212529] dark:text-white block mb-1.5">Password</label><input v-model="pass" type="password" required autocomplete="current-password" /></div>
          <p v-if="err" class="text-xs text-white bg-[#e03131] rounded p-2.5">{{ err }}</p>
          <button type="submit" class="btn btn-primary w-full py-2.5 text-sm font-semibold">Sign in</button>
        </form>
      </div>
      <p class="text-center mt-4 text-xs text-[#6c757d]">Don't have an account? <RouterLink to="/auth/register">Register</RouterLink></p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"; import { useRouter } from "vue-router"; import { useAuthStore } from "../stores/auth";
const r = useRouter(); const a = useAuthStore(); const email = ref(""); const pass = ref(""); const err = ref("");
async function go() { try { const u = await a.login(email.value, pass.value); if (u.must_change_password) r.push("/auth/change-password"); else r.push("/"); } catch (e: any) { err.value = e.message; } }
</script>
