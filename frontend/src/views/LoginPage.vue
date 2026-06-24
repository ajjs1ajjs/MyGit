<template>
  <div class="min-h-screen flex items-center justify-center bg-[#f6f8fa] dark:bg-[#0d1117] p-4">
    <div class="w-full max-w-[320px]">
      <div class="text-center mb-6">
        <RouterLink to="/" class="text-2xl font-bold text-[#24292f] dark:text-white no-underline">MyGit</RouterLink>
        <p class="text-sm text-[#656d76] mt-1">Sign in to your account</p>
      </div>
      <div class="bg-white dark:bg-[#161b22] border border-[#e1e4e8] dark:border-[#30363d] rounded-xl shadow-sm p-5">
        <form @submit.prevent="handle" class="space-y-3">
          <div>
            <label class="text-xs font-medium text-[#24292f] dark:text-[#e6edf3] block mb-1">Email address</label>
            <input v-model="email" type="text" required />
          </div>
          <div>
            <label class="text-xs font-medium text-[#24292f] dark:text-[#e6edf3] block mb-1">Password</label>
            <input v-model="password" type="password" required />
          </div>
          <p v-if="error" class="text-[#cf222e] text-xs bg-[#ffebe9] border border-[#ff818240] rounded-md p-3">{{ error }}</p>
          <button type="submit" class="btn w-full py-1.5 justify-center text-sm font-semibold" style="background:#1f883d;color:#fff;border-color:#1f883d;">Sign in</button>
        </form>
      </div>
      <div class="text-center mt-4 text-xs text-[#656d76]">
        New here? <RouterLink to="/auth/register">Create an account</RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"; import { useRouter } from "vue-router"; import { useAuthStore } from "../stores/auth";
const router = useRouter(); const auth = useAuthStore();
const email = ref(""); const password = ref(""); const error = ref("");
async function handle(){ try{await auth.login(email.value,password.value);router.push("/")}catch(e:any){error.value=e.message} }
</script>
