<template>
  <div class="min-h-screen bg-gray-100 dark:bg-gray-950 flex items-center justify-center p-4">
    <div class="w-full max-w-sm">
      <h2 class="text-center text-lg font-semibold mb-6">Change your password</h2>
      <div class="card">
        <div class="card-body !p-6">
          <p class="text-sm text-gray-500 mb-4">You must change your password before continuing.</p>
          <form @submit.prevent="handle" class="space-y-3">
            <div><label class="text-xs font-medium block mb-1">Current password</label><input v-model="cur" type="password" required /></div>
            <div><label class="text-xs font-medium block mb-1">New password (min 8 chars)</label><input v-model="np" type="password" required minlength="8" /></div>
            <div><label class="text-xs font-medium block mb-1">Confirm new password</label><input v-model="cp" type="password" required /></div>
            <p v-if="msg" class="text-xs bg-red-50 border border-red-200 text-red-600 rounded p-2">{{ msg }}</p>
            <p v-if="ok" class="text-xs bg-green-50 border border-green-200 text-green-600 rounded p-2">Password changed. Redirecting...</p>
            <button type="submit" :disabled="busy" class="btn btn-accent w-full !py-2.5 !text-sm !font-semibold">
              {{ busy ? 'Saving...' : 'Change password' }}
            </button>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"; import { useRouter } from "vue-router"; import { useAuthStore } from "../stores/auth"; import { api } from "../api/client";
const router = useRouter(); const auth = useAuthStore();
const cur=ref(""); const np=ref(""); const cp=ref(""); const msg=ref(""); const ok=ref(false); const busy=ref(false);
async function handle() {
  msg.value = ""; ok.value = false;
  if (np.value !== cp.value) { msg.value = "Passwords do not match."; return; }
  if (np.value.length < 8) { msg.value = "Password must be at least 8 characters."; return; }
  busy.value = true;
  try {
    await api.post("/users/change_password/", { current_password: cur.value, new_password: np.value });
    if (auth.user) auth.user.must_change_password = false;
    ok.value = true;
    setTimeout(() => router.push("/"), 600);
  } catch (e: any) { msg.value = e.message || "Failed. Check current password."; }
  busy.value = false;
}
</script>
