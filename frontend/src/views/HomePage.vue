<template>
  <div class="max-w-5xl">
    <h1 class="text-2xl font-bold mb-6">Welcome to MyGit</h1>

    <div v-if="!auth.user" class="card mb-6">
      <h3 class="font-semibold mb-2">Get started</h3>
      <div class="flex gap-3">
        <RouterLink to="/auth/login" class="btn btn-primary">Sign in</RouterLink>
        <RouterLink to="/auth/register" class="btn btn-default">Create account</RouterLink>
      </div>
    </div>

    <div v-if="auth.user" class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div class="lg:col-span-2">
        <div class="card" v-if="activity.length">
          <h3 class="font-semibold mb-3">Recent activity</h3>
          <div v-for="(a, i) in activity" :key="i" class="py-2 border-b last:border-0 text-sm flex items-center gap-2">
            <span v-if="a.type === 'issue'" class="text-blue-600">🎫</span>
            <span v-else-if="a.type === 'mr'" class="text-green-600">🔀</span>
            <span v-else class="text-purple-600">📝</span>
            {{ a.title }}
            <span class="text-gray-400 text-xs ml-auto">{{ a.repo }}</span>
          </div>
        </div>
        <div v-else class="card text-sm text-gray-500">No recent activity. Create your first project!</div>
      </div>
      <div>
        <div class="card mb-4">
          <div class="flex justify-between items-center mb-3">
            <h3 class="font-semibold">Your projects</h3>
            <button @click="showNew = true" class="btn btn-primary btn-sm">+ New</button>
          </div>
          <div v-if="repos.length">
            <RouterLink v-for="r in repos" :key="r.id" :to="`/${r.path}`" class="block px-2 py-1.5 rounded text-sm hover:bg-gray-50 dark:hover:bg-slate-800 no-underline text-gray-700 dark:text-gray-300">
              📁 {{ r.name }}
            </RouterLink>
          </div>
          <div v-else class="text-sm text-gray-500">No projects yet</div>
        </div>
        <div class="card">
          <h3 class="font-semibold mb-2 text-sm">Quick stats</h3>
          <div class="grid grid-cols-2 gap-2 text-xs">
            <div class="border rounded p-2 text-center"><div class="font-bold text-lg">{{ repos.length }}</div>projects</div>
            <div class="border rounded p-2 text-center"><div class="font-bold text-lg">{{ groups.length }}</div>groups</div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showNew" class="fixed inset-0 bg-black/40 z-50 flex items-center justify-center" @click.self="showNew=false">
      <div class="card w-full max-w-md mx-4">
        <h3 class="font-semibold mb-3">New project</h3>
        <input v-model="newName" placeholder="Project name" class="w-full mb-3" @keyup.enter="createProject" />
        <div class="flex gap-2">
          <button @click="createProject" :disabled="!newName" class="btn btn-primary">Create</button>
          <button @click="showNew = false" class="btn btn-default">Cancel</button>
        </div>
        <p v-if="newError" class="text-red-500 text-xs mt-2">{{ newError }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { api } from "../api/client";

const router = useRouter();
const auth = useAuthStore();
const repos = ref<any[]>([]);
const groups = ref<any[]>([]);
const activity = ref<any[]>([]);
const showNew = ref(false);
const newName = ref("");
const newError = ref("");

async function createProject() {
  if (!newName.value) return;
  try {
    const r = await api.post("/projects/", { name: newName.value, visibility: "public" });
    showNew.value = false;
    newName.value = "";
    router.push(`/${r.path}`);
  } catch (e: any) { newError.value = e.message; }
}

onMounted(async () => {
  if (!auth.user) return;
  try {
    repos.value = (await api.get("/projects/"))?.filter((r: any) => r.path?.startsWith(auth.user!.username + "/")) || [];
    groups.value = (await api.get("/groups/")) || [];
    const iss = (await api.get(`/projects/${repos.value[0]?.id}/issues/`)) || [];
    activity.value = iss.slice(0, 5).map((i: any) => ({ type: "issue", title: i.title, repo: repos.value[0]?.name }));
  } catch {}
});
</script>
