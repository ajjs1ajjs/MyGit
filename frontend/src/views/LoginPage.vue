<template>
  <div class="min-h-screen bg-[#f5f5f5] dark:bg-[#0a0a0a] flex items-center justify-center p-4">
    <div class="w-full max-w-sm">
      <div class="text-center mb-8">
        <div class="text-[#171717] dark:text-[#fafafa] font-bold text-2xl tracking-tight">MyGit</div>
        <p class="text-sm text-[#737373] mt-2">Sign in to your self-hosted Git platform</p>
      </div>
      <div class="card">
        <div class="card-body !p-6">
          <form @submit.prevent="go" class="space-y-4">
            <div><label class="text-xs font-medium text-[#171717] dark:text-[#fafafa] block mb-1.5">Username or email</label><input v-model="email" type="text" required autocomplete="username" placeholder="admin" /></div>
            <div><label class="text-xs font-medium text-[#171717] dark:text-[#fafafa] block mb-1.5">Password</label><input v-model="pass" type="password" required autocomplete="current-password" /></div>
            <p v-if="err" class="text-xs text-[#dc2626] bg-[#fef2f2] border border-[#fecaca] rounded-md p-3">{{ err }}</p>
            <button type="submit" class="btn w-full py-2.5 text-sm font-semibold" style="background:#171717;color:#fafafa">Sign in</button>
          </form>
        </div>
      </div>
      <p class="text-center mt-6 text-xs text-[#737373]">Don't have an account? <RouterLink to="/auth/register" class="font-medium">Create one</RouterLink></p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"; import { useRouter } from "vue-router"; import { useAuthStore } from "../stores/auth";
const r = useRouter(); const a = useAuthStore(); const email=ref(""); const pass=ref(""); const err=ref("");
async function go(){ try{ const u = await a.login(email.value,pass.value); if(u.must_change_password) r.push("/auth/change-password"); else r.push("/") } catch(e:any){ err.value=e.message } }
</script>
