<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-50 px-4">
    <div class="w-full max-w-[960px] flex items-center gap-12">
      <!-- Left: branding -->
      <div class="hidden md:block flex-1">
        <div class="mb-6">
          <div class="text-[#7b58cf] text-3xl font-bold mb-2">MyGit</div>
          <div class="text-gray-500 text-sm">Open source software to collaborate on code</div>
        </div>
        <div class="text-sm text-gray-600 space-y-2">
          <div class="flex items-center gap-2"><span class="text-green-600">&#10003;</span> Manage Git repositories with fine-grained access controls</div>
          <div class="flex items-center gap-2"><span class="text-green-600">&#10003;</span> Code review, issue tracking, merge requests</div>
          <div class="flex items-center gap-2"><span class="text-green-600">&#10003;</span> Self-hosted on your own server</div>
        </div>
      </div>

      <!-- Right: login card -->
      <div class="w-full max-w-sm">
        <div class="md:hidden text-center mb-6">
          <div class="text-[#7b58cf] text-2xl font-bold">MyGit</div>
        </div>
        <div class="bg-white border border-gray-200 rounded p-6 shadow-sm">
          <div class="nav-tabs mb-5">
            <button @click="tab='standard'" class="nav-tab" :class="{ active: tab==='standard' }">Standard</button>
          </div>
          <form @submit.prevent="handle" class="space-y-4">
            <div>
              <label class="text-sm font-medium text-gray-700 block mb-1">Username or email</label>
              <input v-model="email" type="text" required class="w-full" autocomplete="username" />
            </div>
            <div>
              <label class="text-sm font-medium text-gray-700 block mb-1">Password</label>
              <input v-model="password" type="password" required class="w-full" autocomplete="current-password" />
            </div>
            <div class="flex items-center gap-2">
              <input type="checkbox" id="remember" v-model="remember" class="w-auto" />
              <label for="remember" class="text-sm text-gray-600">Remember me</label>
            </div>
            <p v-if="error" class="text-red-600 text-sm bg-red-50 border border-red-200 rounded p-3">{{ error }}</p>
            <button type="submit" class="btn w-full py-2.5 text-base" style="background:#7b58cf;color:#fff;border:none;font-weight:500;">
              Sign in
            </button>
          </form>
        </div>
        <div class="flex gap-4 justify-center mt-4 text-xs text-gray-500">
          <a href="#">Explore</a>
          <a href="#">Help</a>
          <RouterLink to="/auth/register">Register</RouterLink>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"; import { useRouter } from "vue-router"; import { useAuthStore } from "../stores/auth";
const router = useRouter(); const auth = useAuthStore();
const email = ref(""); const password = ref(""); const remember = ref(false); const error = ref(""); const tab = ref("standard");
async function handle() {
  try { await auth.login(email.value, password.value); router.push("/"); }
  catch (e: any) { error.value = e.message; }
}
</script>
