<template>
  <div class="min-h-screen bg-[#f5f5f5] dark:bg-[#0a0a0a] flex items-center justify-center p-4">
    <div class="w-full max-w-sm space-y-6">
      <div class="text-center">
        <div class="text-[#171717] dark:text-[#fafafa] font-bold text-3xl tracking-tight">MyGit</div>
        <p class="text-sm text-[#737373] mt-2">Sign in to your self-hosted Git platform</p>
      </div>

      <div class="card bg-white dark:bg-[#111] border border-[#e5e5e5] dark:border-slate-800 rounded-xl shadow-lg">
        <div class="card-body !p-6 space-y-4">
          <!-- Login Type Tabs -->
          <div class="flex border-b border-[#e5e5e5] dark:border-slate-800 pb-1 gap-2">
            <button 
              type="button"
              @click="loginType = 'standard'"
              :class="loginType === 'standard' ? 'border-b-2 border-emerald-500 font-semibold text-[#171717] dark:text-[#fafafa]' : 'text-[#737373]'"
              class="flex-1 pb-2 text-xs text-center transition"
            >
              Стандартний вхід
            </button>
            <button 
              type="button"
              @click="loginType = 'ldap'"
              :class="loginType === 'ldap' ? 'border-b-2 border-emerald-500 font-semibold text-[#171717] dark:text-[#fafafa]' : 'text-[#737373]'"
              class="flex-1 pb-2 text-xs text-center transition"
            >
              Доменний вхід (LDAP)
            </button>
          </div>

          <form @submit.prevent="go" class="space-y-4">
            <div>
              <label for="login-username" class="text-xs font-medium text-[#171717] dark:text-[#fafafa] block mb-1.5">
                {{ loginType === 'ldap' ? 'Доменне ім\'я (LDAP Username)' : 'Username / Email' }}
              </label>
              <input 
                id="login-username" 
                name="username" 
                v-model="login" 
                type="text" 
                required 
                autocomplete="username" 
                :placeholder="loginType === 'ldap' ? 'uid або sAMAccountName' : 'admin'" 
              />
            </div>
            <div>
              <label for="current-password" class="text-xs font-medium text-[#171717] dark:text-[#fafafa] block mb-1.5">Password</label>
              <input id="current-password" name="password" v-model="pass" type="password" required autocomplete="current-password" />
            </div>

            <!-- LDAP Notice -->
            <div v-if="loginType === 'ldap'" class="bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-900/50 rounded-lg p-3 text-xs text-blue-700 dark:text-blue-300">
              💡 Ваш обліковий запис буде створено автоматично при першому вході. Після цього адміністратор зможе надати вам доступ до груп та проектів.
            </div>

            <p v-if="err" class="text-xs text-[#dc2626] bg-[#fef2f2] border border-[#fecaca] rounded-md p-3">{{ err }}</p>
            <button type="submit" class="btn w-full py-2.5 text-sm font-semibold btn-accent">
              Sign in
            </button>
          </form>
        </div>
      </div>
      <p class="text-center text-xs text-[#737373]">Don't have an account? <RouterLink to="/auth/register" class="font-medium hover:underline">Create one</RouterLink></p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"; 
import { useRouter } from "vue-router"; 
import { useAuthStore } from "../stores/auth";

const r = useRouter(); 
const a = useAuthStore(); 
const login = ref(""); 
const pass = ref(""); 
const err = ref("");
const loginType = ref<"standard" | "ldap">("standard");

async function go() { 
  err.value = "";
  try { 
    const u = await a.login(login.value, pass.value); 
    if (u.must_change_password) {
      r.push("/auth/change-password"); 
    } else {
      r.push("/"); 
    }
  } catch (e: any) { 
    err.value = e.message; 
  } 
}
</script>
