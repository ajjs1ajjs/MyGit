<template>
  <div class="max-w-4xl mx-auto py-6">
    <!-- Header -->
    <div class="flex items-center gap-4 mb-6">
      <div class="w-16 h-16 rounded-full bg-blue-500 flex items-center justify-center text-white text-xl font-bold">
        {{ initials }}
      </div>
      <div>
        <h1 class="text-2xl font-bold">{{ profile?.full_name || username }}</h1>
        <p class="text-sm text-gray-500">@{{ username }}</p>
        <p v-if="profile?.bio" class="text-sm text-gray-600 dark:text-gray-300 mt-1">{{ profile.bio }}</p>
      </div>
    </div>

    <!-- Tabs (Only if owner) -->
    <div v-if="isOwner" class="navtabs mb-6">
      <button 
        @click="activeTab = 'repos'" 
        :class="{ active: activeTab === 'repos' }"
        class="border-0 bg-transparent cursor-pointer font-inherit px-4 py-2"
      >
        Repositories
      </button>
      <button 
        @click="activeTab = 'keys'" 
        :class="{ active: activeTab === 'keys' }"
        class="border-0 bg-transparent cursor-pointer font-inherit px-4 py-2"
      >
        SSH Keys
      </button>
      <button 
        @click="activeTab = 'tokens'" 
        :class="{ active: activeTab === 'tokens' }"
        class="border-0 bg-transparent cursor-pointer font-inherit px-4 py-2"
      >
        Personal Access Tokens
      </button>
      <button 
        @click="activeTab = 'integrations'" 
        :class="{ active: activeTab === 'integrations' }"
        class="border-0 bg-transparent cursor-pointer font-inherit px-4 py-2"
      >
        Integrations
      </button>
    </div>

    <!-- Tab Contents -->
    <div class="card p-6">
      <!-- 1. Repositories Tab -->
      <div v-if="activeTab === 'repos'" class="space-y-4">
        <h2 class="font-semibold text-lg mb-3">Repositories</h2>
        <div v-if="repos.length" class="grid gap-3">
          <RouterLink v-for="r in repos" :key="r.id" :to="`/${r.path}`" class="p-4 border rounded-lg hover:shadow block !no-underline group">
            <div class="flex items-center justify-between">
              <h3 class="font-semibold group-hover:text-blue-600">{{ r.name }}</h3>
              <span class="badge" :class="r.visibility === 'public' ? 'badge-blue' : 'badge-gray'">{{ r.visibility }}</span>
            </div>
            <p class="text-sm text-gray-500 mt-1">{{ r.description || r.path }}</p>
            <div class="text-xs text-gray-400 mt-2">{{ new Date(r.updated_at).toLocaleDateString() }}</div>
          </RouterLink>
        </div>
        <p v-else-if="!loading" class="text-gray-500 text-sm">No repositories yet.</p>
        <p v-if="loading" class="text-sm text-gray-500">Loading...</p>
      </div>

      <!-- 2. SSH Keys Tab -->
      <div v-if="activeTab === 'keys'" class="space-y-6">
        <div>
          <h2 class="font-semibold text-lg mb-1">SSH Keys</h2>
          <p class="text-sm text-gray-500">Add SSH keys to authenticate secure Git operations over SSH.</p>
        </div>

        <!-- Add SSH Key Form -->
        <div class="p-4 border rounded-lg bg-gray-50 dark:bg-slate-800 space-y-3 max-w-lg">
          <h3 class="font-medium text-sm">Add SSH Key</h3>
          <div>
            <label class="text-xs font-semibold mb-1 block">Title</label>
            <input v-model="sshTitle" placeholder="My laptop" />
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Key</label>
            <textarea v-model="sshKey" placeholder="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQD..." rows="4" class="font-mono text-xs"></textarea>
          </div>
          <p v-if="sshError" class="text-xs text-[#dc2626]">{{ sshError }}</p>
          <button @click="addSshKey" :disabled="!sshTitle || !sshKey || subLoading" class="btn btn-accent btn-sm">Add key</button>
        </div>

        <!-- SSH Keys List -->
        <div class="space-y-3">
          <h3 class="font-semibold text-sm">Your SSH Keys</h3>
          <div v-if="sshKeys.length" class="border rounded-lg divide-y">
            <div v-for="key in sshKeys" :key="key.id" class="p-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-slate-800">
              <div class="min-w-0 pr-4">
                <div class="font-medium text-sm">{{ key.title }}</div>
                <div class="text-xs text-gray-400 font-mono mt-0.5 truncate">{{ key.fingerprint }}</div>
                <div class="text-[10px] text-gray-500 mt-0.5">Added on {{ new Date(key.created_at).toLocaleDateString() }}</div>
              </div>
              <button @click="deleteSshKey(key.id)" class="btn btn-danger btn-sm">Delete</button>
            </div>
          </div>
          <p v-else class="text-sm text-gray-500">No SSH keys added yet.</p>
        </div>
      </div>

      <!-- 3. Personal Access Tokens Tab -->
      <div v-if="activeTab === 'tokens'" class="space-y-6">
        <div>
          <h2 class="font-semibold text-lg mb-1">Personal Access Tokens</h2>
          <p class="text-sm text-gray-500">Create personal access tokens to authenticate with the MyGit API.</p>
        </div>

        <!-- Add PAT Form -->
        <div class="p-4 border rounded-lg bg-gray-50 dark:bg-slate-800 space-y-3 max-w-lg">
          <h3 class="font-medium text-sm">Generate new token</h3>
          <div>
            <label class="text-xs font-semibold mb-1 block">Token name</label>
            <input v-model="patName" placeholder="VSCode integration" />
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Scopes</label>
            <div class="space-y-1.5 mt-1">
              <label class="flex items-center gap-2 text-xs cursor-pointer">
                <input type="checkbox" v-model="patScopes" value="read_repo" class="w-auto" />
                <span>read_repo (Read repositories)</span>
              </label>
              <label class="flex items-center gap-2 text-xs cursor-pointer">
                <input type="checkbox" v-model="patScopes" value="write_repo" class="w-auto" />
                <span>write_repo (Write repositories)</span>
              </label>
              <label class="flex items-center gap-2 text-xs cursor-pointer">
                <input type="checkbox" v-model="patScopes" value="api" class="w-auto" />
                <span>api (Full API access)</span>
              </label>
            </div>
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Expiration (days)</label>
            <input v-model.number="patExpiresDays" type="number" placeholder="30 (leave empty for no expiration)" />
          </div>
          
          <p v-if="patError" class="text-xs text-[#dc2626]">{{ patError }}</p>
          <button @click="createPat" :disabled="!patName || subLoading" class="btn btn-accent btn-sm">Generate token</button>
        </div>

        <!-- Newly generated token warning -->
        <div v-if="newPatValue" class="p-4 border border-yellow-200 bg-yellow-50 dark:bg-amber-950 dark:border-amber-900 rounded-lg space-y-2">
          <div class="font-semibold text-sm text-yellow-800 dark:text-amber-300">Make sure to copy your personal access token now!</div>
          <p class="text-xs text-yellow-700 dark:text-amber-400">You won't be able to see it again.</p>
          <div class="flex gap-2">
            <input :value="newPatValue" readonly class="font-mono text-xs bg-white dark:bg-slate-900 flex-1 border border-yellow-300" @click="selectInput" />
            <button @click="copyToClipboard(newPatValue)" class="btn btn-ghost btn-sm bg-white border">Copy</button>
          </div>
        </div>

        <!-- PAT List -->
        <div class="space-y-3">
          <h3 class="font-semibold text-sm">Active Tokens</h3>
          <div v-if="patTokens.length" class="border rounded-lg divide-y">
            <div v-for="token in patTokens" :key="token.id" class="p-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-slate-800">
              <div class="min-w-0 pr-4">
                <div class="font-medium text-sm flex items-center gap-2">
                  {{ token.name }}
                  <span v-for="sc in token.scopes" :key="sc" class="badge badge-gray text-[9px]">{{ sc }}</span>
                </div>
                <div class="text-[10px] text-gray-500 mt-1">
                  Created on {{ new Date(token.created_at).toLocaleDateString() }} 
                  <span v-if="token.expires_at"> &middot; Expires {{ new Date(token.expires_at).toLocaleDateString() }}</span>
                  <span v-else> &middot; Never expires</span>
                </div>
              </div>
              <button @click="deletePat(token.id)" class="btn btn-danger btn-sm">Revoke</button>
            </div>
          </div>
          <p v-else class="text-sm text-gray-500">No active personal access tokens found.</p>
        </div>
      </div>

      <!-- 4. Integrations Tab -->
      <div v-if="activeTab === 'integrations'" class="space-y-6">
        <div>
          <h2 class="font-semibold text-lg mb-1">Integrations</h2>
          <p class="text-sm text-gray-500">Configure GitHub and GitLab Personal Access Tokens to import external projects.</p>
        </div>

        <div class="grid md:grid-cols-2 gap-6">
          <!-- GitHub Integration Card -->
          <div class="border rounded-lg p-4 bg-gray-50 dark:bg-slate-800 space-y-3">
            <div class="flex justify-between items-start">
              <div>
                <h3 class="font-semibold">GitHub Connection</h3>
                <span class="badge mt-1" :class="githubConnected ? 'badge-green' : 'badge-gray'">
                  {{ githubConnected ? 'Connected' : 'Not Connected' }}
                </span>
              </div>
              <button v-if="githubConnected" @click="deleteIntegration('github')" class="btn btn-danger btn-sm">Disconnect</button>
            </div>
            
            <div v-if="!githubConnected" class="space-y-2 pt-2">
              <input v-model="githubInput" type="password" placeholder="GitHub Personal Access Token" />
              <button @click="saveIntegration('github')" :disabled="!githubInput || subLoading" class="btn btn-accent btn-sm w-full">Connect GitHub</button>
            </div>
            <div v-else class="text-xs text-gray-500 pt-2">
              Connected with token: <code class="font-mono">{{ githubMasked }}</code>
            </div>
          </div>

          <!-- GitLab Integration Card -->
          <div class="border rounded-lg p-4 bg-gray-50 dark:bg-slate-800 space-y-3">
            <div class="flex justify-between items-start">
              <div>
                <h3 class="font-semibold">GitLab Connection</h3>
                <span class="badge mt-1" :class="gitlabConnected ? 'badge-green' : 'badge-gray'">
                  {{ gitlabConnected ? 'Connected' : 'Not Connected' }}
                </span>
              </div>
              <button v-if="gitlabConnected" @click="deleteIntegration('gitlab')" class="btn btn-danger btn-sm">Disconnect</button>
            </div>
            
            <div v-if="!gitlabConnected" class="space-y-2 pt-2">
              <input v-model="gitlabInput" type="password" placeholder="GitLab Personal Access Token" />
              <button @click="saveIntegration('gitlab')" :disabled="!gitlabInput || subLoading" class="btn btn-accent btn-sm w-full">Connect GitLab</button>
            </div>
            <div v-else class="text-xs text-gray-500 pt-2">
              Connected with token: <code class="font-mono">{{ gitlabMasked }}</code>
            </div>
          </div>
        </div>

        <p v-if="integrationError" class="text-xs text-[#dc2626]">{{ integrationError }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { api } from "../api/client";

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();

const username = route.params.username as string;
const profile = ref<any>(null);
const repos = ref<any[]>([]);
const loading = ref(true);
const subLoading = ref(false);

const activeTab = ref("repos");

// SSH Keys State
const sshKeys = ref<any[]>([]);
const sshTitle = ref("");
const sshKey = ref("");
const sshError = ref("");

// PAT State
const patTokens = ref<any[]>([]);
const patName = ref("");
const patScopes = ref<string[]>([]);
const patExpiresDays = ref<number | null>(null);
const patError = ref("");
const newPatValue = ref("");

// Integrations State
const githubConnected = ref(false);
const githubMasked = ref("");
const githubInput = ref("");
const gitlabConnected = ref(false);
const gitlabMasked = ref("");
const gitlabInput = ref("");
const integrationError = ref("");

const isOwner = computed(() => {
  return auth.user && auth.user.username === username;
});

const initials = computed(() => {
  const name = profile.value?.full_name || profile.value?.username || username;
  return name.slice(0, 2).toUpperCase();
});

onMounted(async () => {
  try {
    const profData = await api.get(`/users/${username}/`);
    profile.value = profData;
    
    // Load repositories
    const repoData = await api.get("/projects/");
    repos.value = (repoData || []).filter((r: any) => r.path?.startsWith(username + "/"));
  } catch (e) {
    console.error("Failed to load profile", e);
  }
  loading.value = false;
});

// Watch tab switches to fetch dynamic data
watch(activeTab, (tab) => {
  if (!isOwner.value) return;
  if (tab === "keys") fetchSshKeys();
  if (tab === "tokens") fetchPats();
  if (tab === "integrations") fetchIntegrations();
});

// SSH Keys Logic
async function fetchSshKeys() {
  try {
    sshKeys.value = await api.get(`/users/${username}/keys/`) || [];
  } catch {}
}

async function addSshKey() {
  if (!sshTitle.value || !sshKey.value) return;
  subLoading.value = true;
  sshError.value = "";
  try {
    await api.post(`/users/${username}/keys/`, {
      title: sshTitle.value,
      public_key: sshKey.value
    });
    sshTitle.value = "";
    sshKey.value = "";
    await fetchSshKeys();
  } catch (e: any) {
    sshError.value = e.message || "Failed to add SSH key.";
  } finally {
    subLoading.value = false;
  }
}

async function deleteSshKey(id: string) {
  if (!confirm("Are you sure you want to delete this SSH Key?")) return;
  try {
    await api.delete(`/users/${username}/keys/${id}/`);
    await fetchSshKeys();
  } catch {}
}

// PAT Logic
async function fetchPats() {
  try {
    patTokens.value = await api.get(`/users/${username}/tokens/`) || [];
  } catch {}
}

async function createPat() {
  if (!patName.value) return;
  subLoading.value = true;
  patError.value = "";
  newPatValue.value = "";
  try {
    const res = await api.post(`/users/${username}/tokens/`, {
      name: patName.value,
      scopes: patScopes.value,
      expires_in_days: patExpiresDays.value || null
    });
    patName.value = "";
    patScopes.value = [];
    patExpiresDays.value = null;
    newPatValue.value = res.token;
    await fetchPats();
  } catch (e: any) {
    patError.value = e.message || "Failed to generate personal access token.";
  } finally {
    subLoading.value = false;
  }
}

async function deletePat(id: string) {
  if (!confirm("Are you sure you want to revoke this Personal Access Token?")) return;
  try {
    await api.delete(`/users/${username}/tokens/${id}/`);
    await fetchPats();
  } catch {}
}

// Integrations Logic
async function fetchIntegrations() {
  try {
    const tokens = await api.get(`/users/${username}/integration-tokens/`) || [];
    
    const gh = tokens.find((t: any) => t.provider === "github");
    githubConnected.value = !!gh;
    githubMasked.value = gh ? gh.masked_token : "";

    const gl = tokens.find((t: any) => t.provider === "gitlab");
    gitlabConnected.value = !!gl;
    gitlabMasked.value = gl ? gl.masked_token : "";
  } catch {}
}

async function saveIntegration(provider: string) {
  const tokenVal = provider === "github" ? githubInput.value : gitlabInput.value;
  if (!tokenVal) return;
  
  subLoading.value = true;
  integrationError.value = "";
  try {
    await api.post(`/users/${username}/integration-tokens/`, {
      provider,
      token: tokenVal
    });
    if (provider === "github") {
      githubInput.value = "";
    } else {
      gitlabInput.value = "";
    }
    await fetchIntegrations();
  } catch (e: any) {
    integrationError.value = e.message || "Failed to save integration token.";
  } finally {
    subLoading.value = false;
  }
}

async function deleteIntegration(provider: string) {
  if (!confirm(`Are you sure you want to disconnect your ${provider === "github" ? "GitHub" : "GitLab"} integration?`)) return;
  try {
    await api.delete(`/users/${username}/integration-tokens/${provider}/`);
    await fetchIntegrations();
  } catch (e: any) {
    integrationError.value = "Failed to delete integration.";
  }
}

function copyToClipboard(val: string) {
  navigator.clipboard.writeText(val);
  alert("Token copied to clipboard!");
}

function selectInput(event: Event) {
  const target = event.target as HTMLInputElement;
  if (target) {
    target.select();
  }
}
</script>
