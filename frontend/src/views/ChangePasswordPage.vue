<template>
  <div class="min-h-screen bg-[#f8f9fa] flex items-center justify-center p-4">
    <div class="w-full max-w-sm">
      <h2 class="text-center text-lg font-semibold mb-6">Change your password</h2>
      <div class="bg-white dark:bg-[#16213e] border border-[#dee2e6] dark:border-[#2a2a4a] rounded-xl p-6 shadow-sm">
        <p class="text-sm text-[#6c757d] mb-4">You must change your password before continuing.</p>
        <form @submit.prevent="handle" class="space-y-3">
          <div><label class="text-xs font-medium block mb-1">Current password</label><input v-model="current" type="password" required /></div>
          <div><label class="text-xs font-medium block mb-1">New password</label><input v-model="newPass" type="password" required minlength="8" /></div>
          <div><label class="text-xs font-medium block mb-1">Confirm new password</label><input v-model="confirm" type="password" required /></div>
          <p v-if="error" class="text-[#c92a2a] text-xs bg-[#ffe3e3] rounded p-2">{{ error }}</p>
          <button type="submit" :disabled="loading" class="btn btn-primary w-full">Change password</button>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"; import { useRouter } from "vue-router"; import { api } from "../api/client";
const router = useRouter();
const current = ref(""); const newPass = ref(""); const confirm = ref(""); const error = ref(""); const loading = ref(false);
async function handle() {
  if (newPass.value !== confirm.value) { error.value = "Passwords do not match."; return; }
  loading.value = true; error.value = "";
  try { await api.post("/users/change_password/", { current_password: current.value, new_password: newPass.value }); router.push("/"); }
  catch (e: any) { error.value = e.message; }
  loading.value = false;
}
</script>
