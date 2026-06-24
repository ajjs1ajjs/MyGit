<template>
  <div class="max-w-4xl mx-auto py-6">
    <div class="flex items-center gap-2 mb-6">
      <RouterLink to="/" class="text-sm hover:underline text-[#737373]">&larr; Back to projects</RouterLink>
    </div>
    
    <div class="flex justify-between items-center mb-6">
      <h1 class="text-2xl font-bold">New project</h1>
    </div>

    <!-- Tab navigation -->
    <div class="navtabs mb-6">
      <button 
        @click="activeTab = 'blank'" 
        :class="{ active: activeTab === 'blank' }"
        class="border-0 bg-transparent cursor-pointer font-inherit px-4 py-2"
      >
        Blank project
      </button>
      <button 
        @click="activeTab = 'github'" 
        :class="{ active: activeTab === 'github' }"
        class="border-0 bg-transparent cursor-pointer font-inherit px-4 py-2"
      >
        Import from GitHub
      </button>
      <button 
        @click="activeTab = 'gitlab'" 
        :class="{ active: activeTab === 'gitlab' }"
        class="border-0 bg-transparent cursor-pointer font-inherit px-4 py-2"
      >
        Import from GitLab
      </button>
      <button 
        @click="activeTab = 'url'" 
        :class="{ active: activeTab === 'url' }"
        class="border-0 bg-transparent cursor-pointer font-inherit px-4 py-2"
      >
        Import from URL
      </button>
    </div>

    <!-- Tab contents -->
    <div class="card p-6">
      <!-- 1. Blank project -->
      <div v-if="activeTab === 'blank'" class="space-y-4">
        <h3 class="font-semibold text-lg">Create a blank project</h3>
        <p class="text-sm text-[#737373] mb-4">Create a new empty repository to start versioning your code.</p>
        
        <div class="space-y-3 max-w-lg">
          <div class="flex gap-3">
            <div class="w-1/3">
              <label class="text-xs font-semibold mb-1 block">Owner / Namespace</label>
              <select v-model="namespace" class="w-full px-3 py-2 border rounded text-sm bg-white dark:bg-slate-800">
                <option :value="`user:${auth.user?.id}`">{{ auth.user?.username }} (Personal)</option>
                <option v-for="g in groups" :key="g.id" :value="`organization:${g.id}`">{{ g.path }}</option>
              </select>
            </div>
            <div class="flex-1">
              <label class="text-xs font-semibold mb-1 block">Project name</label>
              <input v-model="blankName" placeholder="my-awesome-project" class="w-full" />
            </div>
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Description (optional)</label>
            <textarea v-model="blankDesc" placeholder="Brief explanation of the project" rows="3"></textarea>
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Visibility</label>
            <div class="flex gap-2">
              <button 
                @click="blankVis = 'private'" 
                :class="blankVis === 'private' ? 'btn-accent' : 'btn-ghost'"
                class="btn btn-sm flex-1"
              >
                Private
              </button>
              <button 
                @click="blankVis = 'public'" 
                :class="blankVis === 'public' ? 'btn-accent' : 'btn-ghost'"
                class="btn btn-sm flex-1"
              >
                Public
              </button>
            </div>
          </div>
          
          <p v-if="error" class="text-xs text-[#dc2626]">{{ error }}</p>
          <div class="pt-2">
            <button @click="createBlank" :disabled="!blankName || loading" class="btn btn-accent px-6">
              <span v-if="loading">Creating...</span>
              <span v-else>Create project</span>
            </button>
          </div>
        </div>
      </div>

      <!-- 2. Import from GitHub -->
      <div v-if="activeTab === 'github'" class="space-y-4">
        <div class="flex justify-between items-center">
          <div>
            <h3 class="font-semibold text-lg">Import from GitHub</h3>
            <p class="text-sm text-[#737373]">Connect GitHub using a Personal Access Token to import repositories.</p>
          </div>
          <button v-if="gitHubTokenStatus" @click="disconnectToken('github')" class="btn btn-danger btn-sm">Disconnect</button>
        </div>

        <!-- Token Entry Form -->
        <div v-if="!gitHubTokenStatus" class="max-w-lg p-4 border rounded-lg bg-gray-50 dark:bg-slate-800 space-y-3">
          <p class="text-xs text-[#737373]">
            To import public/private repositories, generate a Personal Access Token (classic) with <code>repo</code> scope, or a Fine-Grained token with Read access to repositories on GitHub.
          </p>
          <div class="flex gap-2">
            <input v-model="gitHubTokenInput" type="password" placeholder="GitHub Personal Access Token" />
            <button @click="saveToken('github')" :disabled="!gitHubTokenInput || loading" class="btn btn-accent">Connect</button>
          </div>
          <p v-if="tokenError" class="text-xs text-[#dc2626]">{{ tokenError }}</p>
        </div>

        <!-- Repo list -->
        <div v-else class="space-y-4">
          <div class="flex gap-2">
            <input v-model="githubSearch" placeholder="Search GitHub repositories..." class="flex-1" />
            <button @click="fetchRepos('github')" :disabled="loading" class="btn btn-ghost">
              Refresh list
            </button>
          </div>

          <div v-if="loading && !githubRepos.length" class="text-center py-6 text-sm text-[#737373]">
            Fetching repositories from GitHub...
          </div>
          
          <div v-else-if="githubFiltered.length" class="border rounded-lg divide-y max-h-96 overflow-y-auto">
            <div v-for="r in githubFiltered" :key="r.full_name" class="p-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-slate-800">
              <div class="min-w-0 pr-4">
                <div class="font-medium text-sm truncate flex items-center gap-1.5">
                  {{ r.name }}
                  <span class="badge badge-gray text-[10px]">{{ r.private ? 'Private' : 'Public' }}</span>
                </div>
                <div class="text-xs text-[#737373] truncate mt-0.5">{{ r.description || 'No description' }}</div>
              </div>
              <button @click="selectImportTarget('github', r)" class="btn btn-accent btn-sm">Import</button>
            </div>
          </div>
          
          <div v-else class="text-center py-6 text-sm text-[#737373]">
            No repositories found on GitHub.
          </div>
        </div>
      </div>

      <!-- 3. Import from GitLab -->
      <div v-if="activeTab === 'gitlab'" class="space-y-4">
        <div class="flex justify-between items-center">
          <div>
            <h3 class="font-semibold text-lg">Import from GitLab</h3>
            <p class="text-sm text-[#737373]">Connect GitLab using a Personal Access Token to import projects.</p>
          </div>
          <button v-if="gitLabTokenStatus" @click="disconnectToken('gitlab')" class="btn btn-danger btn-sm">Disconnect</button>
        </div>

        <!-- Token Entry Form -->
        <div v-if="!gitLabTokenStatus" class="max-w-lg p-4 border rounded-lg bg-gray-50 dark:bg-slate-800 space-y-3">
          <p class="text-xs text-[#737373]">
            Generate a GitLab Personal Access Token with <code>read_repository</code> or <code>api</code> scope.
          </p>
          <div class="flex gap-2">
            <input v-model="gitLabTokenInput" type="password" placeholder="GitLab Personal Access Token" />
            <button @click="saveToken('gitlab')" :disabled="!gitLabTokenInput || loading" class="btn btn-accent">Connect</button>
          </div>
          <p v-if="tokenError" class="text-xs text-[#dc2626]">{{ tokenError }}</p>
        </div>

        <!-- Repo list -->
        <div v-else class="space-y-4">
          <div class="flex gap-2">
            <input v-model="gitlabSearch" placeholder="Search GitLab projects..." class="flex-1" />
            <button @click="fetchRepos('gitlab')" :disabled="loading" class="btn btn-ghost">
              Refresh list
            </button>
          </div>

          <div v-if="loading && !gitlabRepos.length" class="text-center py-6 text-sm text-[#737373]">
            Fetching projects from GitLab...
          </div>
          
          <div v-else-if="gitlabFiltered.length" class="border rounded-lg divide-y max-h-96 overflow-y-auto">
            <div v-for="r in gitlabFiltered" :key="r.full_name" class="p-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-slate-800">
              <div class="min-w-0 pr-4">
                <div class="font-medium text-sm truncate flex items-center gap-1.5">
                  {{ r.name }}
                  <span class="badge badge-gray text-[10px]">{{ r.private ? 'Private' : 'Public' }}</span>
                </div>
                <div class="text-xs text-[#737373] truncate mt-0.5">{{ r.description || 'No description' }}</div>
              </div>
              <button @click="selectImportTarget('gitlab', r)" class="btn btn-accent btn-sm">Import</button>
            </div>
          </div>
          
          <div v-else class="text-center py-6 text-sm text-[#737373]">
            No projects found on GitLab.
          </div>
        </div>
      </div>

      <!-- 4. Import from URL -->
      <div v-if="activeTab === 'url'" class="space-y-4">
        <h3 class="font-semibold text-lg">Import repository by URL</h3>
        <p class="text-sm text-[#737373] mb-4">Clone any public or private Git repository using its HTTPS clone URL.</p>
        
        <div class="space-y-3 max-w-lg">
          <div>
            <label class="text-xs font-semibold mb-1 block">Git Repository URL (HTTPS)</label>
            <input v-model="urlCloneUrl" placeholder="https://example.com/some/repo.git" />
          </div>
          <div class="flex gap-2">
            <div class="flex-1">
              <label class="text-xs font-semibold mb-1 block">Username (optional)</label>
              <input v-model="urlUsername" placeholder="git-username" />
            </div>
            <div class="flex-1">
              <label class="text-xs font-semibold mb-1 block">Password / Token (optional)</label>
              <input v-model="urlToken" type="password" placeholder="personal-token-or-password" />
            </div>
          </div>
          <div class="flex gap-3">
            <div class="w-1/3">
              <label class="text-xs font-semibold mb-1 block">Owner / Namespace</label>
              <select v-model="namespace" class="w-full px-3 py-2 border rounded text-sm bg-white dark:bg-slate-800">
                <option :value="`user:${auth.user?.id}`">{{ auth.user?.username }} (Personal)</option>
                <option v-for="g in groups" :key="g.id" :value="`organization:${g.id}`">{{ g.path }}</option>
              </select>
            </div>
            <div class="flex-1">
              <label class="text-xs font-semibold mb-1 block">Local project name</label>
              <input v-model="urlName" placeholder="local-name" class="w-full" />
            </div>
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Description (optional)</label>
            <textarea v-model="urlDesc" placeholder="Brief explanation" rows="2"></textarea>
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Visibility</label>
            <div class="flex gap-2">
              <button 
                @click="urlVis = 'private'" 
                :class="urlVis === 'private' ? 'btn-accent' : 'btn-ghost'"
                class="btn btn-sm flex-1"
              >
                Private
              </button>
              <button 
                @click="urlVis = 'public'" 
                :class="urlVis === 'public' ? 'btn-accent' : 'btn-ghost'"
                class="btn btn-sm flex-1"
              >
                Public
              </button>
            </div>
          </div>
          
          <p v-if="error" class="text-xs text-[#dc2626]">{{ error }}</p>
          <div class="pt-2">
            <button @click="createFromUrl" :disabled="!urlCloneUrl || !urlName || loading" class="btn btn-accent px-6">
              <span v-if="loading">Importing...</span>
              <span v-else>Import project</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Import options modal -->
    <div v-if="showImportModal" class="modal-overlay" @click.self="showImportModal = false">
      <div class="modal">
        <div class="card-header">
          Import repository
          <button @click="showImportModal = false" class="text-[#a3a3a3] hover:text-[#737373] text-lg leading-none">&times;</button>
        </div>
        <div class="card-body space-y-4">
          <div>
            <label class="text-xs font-semibold mb-1 block">Source path</label>
            <input :value="importTarget.full_name" disabled class="bg-gray-100 dark:bg-slate-700 w-full px-3 py-2 border rounded text-sm" />
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Namespace / Owner</label>
            <select v-model="namespace" class="w-full px-3 py-2 border rounded text-sm bg-white dark:bg-slate-800">
              <option :value="`user:${auth.user?.id}`">{{ auth.user?.username }} (Personal)</option>
              <option v-for="g in groups" :key="g.id" :value="`organization:${g.id}`">{{ g.path }}</option>
            </select>
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Local project name</label>
            <input v-model="importLocalName" />
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Description</label>
            <textarea v-model="importDescription" rows="2"></textarea>
          </div>
          <div>
            <label class="text-xs font-semibold mb-1 block">Visibility</label>
            <div class="flex gap-2">
              <button 
                @click="importVisibility = 'private'" 
                :class="importVisibility === 'private' ? 'btn-accent' : 'btn-ghost'"
                class="btn btn-sm flex-1"
              >
                Private
              </button>
              <button 
                @click="importVisibility = 'public'" 
                :class="importVisibility === 'public' ? 'btn-accent' : 'btn-ghost'"
                class="btn btn-sm flex-1"
              >
                Public
              </button>
            </div>
          </div>
          
          <p v-if="error" class="text-xs text-[#dc2626]">{{ error }}</p>
          <div class="flex gap-2 pt-2">
            <button @click="executeImport" :disabled="loading" class="btn btn-accent flex-1">
              <span v-if="loading">Importing...</span>
              <span v-else>Confirm Import</span>
            </button>
            <button @click="showImportModal = false" class="btn btn-ghost">Cancel</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { api } from "../api/client";

const router = useRouter();
const auth = useAuthStore();

const activeTab = ref("blank");
const loading = ref(false);
const error = ref("");
const tokenError = ref("");

const groups = ref<any[]>([]);
const namespace = ref("");

const selectedOwner = computed(() => {
  if (!namespace.value) return { type: "user", id: auth.user?.id || "" };
  const parts = namespace.value.split(":");
  return { type: parts[0], id: parts[1] };
});

// Blank Project fields
const blankName = ref("");
const blankDesc = ref("");
const blankVis = ref("private");

// Integration token state
const gitHubTokenStatus = ref(false);
const gitHubTokenInput = ref("");
const gitLabTokenStatus = ref(false);
const gitLabTokenInput = ref("");

// Remote repos lists
const githubRepos = ref<any[]>([]);
const githubSearch = ref("");
const gitlabRepos = ref<any[]>([]);
const gitlabSearch = ref("");

// URL Import fields
const urlCloneUrl = ref("");
const urlUsername = ref("");
const urlToken = ref("");
const urlName = ref("");
const urlDesc = ref("");
const urlVis = ref("private");

// Import options modal
const showImportModal = ref(false);
const importProvider = ref("");
const importTarget = ref<any>({});
const importLocalName = ref("");
const importDescription = ref("");
const importVisibility = ref("private");

// Search filters
const githubFiltered = computed(() => {
  if (!githubSearch.value) return githubRepos.value;
  return githubRepos.value.filter(r => 
    r.name.toLowerCase().includes(githubSearch.value.toLowerCase()) || 
    r.full_name.toLowerCase().includes(githubSearch.value.toLowerCase())
  );
});

const gitlabFiltered = computed(() => {
  if (!gitlabSearch.value) return gitlabRepos.value;
  return gitlabRepos.value.filter(r => 
    r.name.toLowerCase().includes(gitlabSearch.value.toLowerCase()) || 
    r.full_name.toLowerCase().includes(gitlabSearch.value.toLowerCase())
  );
});

// Watch tabs to load data
watch(activeTab, (tab) => {
  error.value = "";
  tokenError.value = "";
  if (tab === "github" && gitHubTokenStatus.value && !githubRepos.value.length) {
    fetchRepos("github");
  }
  if (tab === "gitlab" && gitLabTokenStatus.value && !gitlabRepos.value.length) {
    fetchRepos("gitlab");
  }
});

// Watch URL input to guess local name
watch(urlCloneUrl, (url) => {
  if (url) {
    try {
      const parts = url.split("/");
      const last = parts[parts.length - 1];
      if (last) {
        urlName.value = last.replace(/\.git$/, "");
      }
    } catch {}
  }
});

onMounted(async () => {
  if (!auth.user) {
    router.push("/auth/login");
    return;
  }
  namespace.value = `user:${auth.user.id}`;
  await checkTokens();
  try {
    groups.value = await api.get("/groups/") || [];
  } catch {}
});

async function checkTokens() {
  try {
    const tokens = await api.get(`/users/${auth.user!.username}/integration-tokens/`);
    gitHubTokenStatus.value = tokens.some((t: any) => t.provider === "github");
    gitLabTokenStatus.value = tokens.some((t: any) => t.provider === "gitlab");
    
    if (activeTab.value === "github" && gitHubTokenStatus.value) {
      fetchRepos("github");
    }
    if (activeTab.value === "gitlab" && gitLabTokenStatus.value) {
      fetchRepos("gitlab");
    }
  } catch (e: any) {
    console.error("Failed to load integrations", e);
  }
}

async function saveToken(provider: string) {
  const tokenVal = provider === "github" ? gitHubTokenInput.value : gitLabTokenInput.value;
  if (!tokenVal) return;
  
  loading.value = true;
  tokenError.value = "";
  try {
    await api.post(`/users/${auth.user!.username}/integration-tokens/`, {
      provider,
      token: tokenVal
    });
    
    if (provider === "github") {
      gitHubTokenInput.value = "";
      gitHubTokenStatus.value = true;
    } else {
      gitLabTokenInput.value = "";
      gitLabTokenStatus.value = true;
    }
    
    await fetchRepos(provider);
  } catch (e: any) {
    tokenError.value = e.message || "Failed to connect integration.";
  } finally {
    loading.value = false;
  }
}

async function disconnectToken(provider: string) {
  if (!confirm(`Are you sure you want to disconnect your ${provider === "github" ? "GitHub" : "GitLab"} integration?`)) {
    return;
  }
  try {
    await api.delete(`/users/${auth.user!.username}/integration-tokens/${provider}/`);
    if (provider === "github") {
      gitHubTokenStatus.value = false;
      githubRepos.value = [];
    } else {
      gitLabTokenStatus.value = false;
      gitlabRepos.value = [];
    }
  } catch (e: any) {
    error.value = "Failed to disconnect token.";
  }
}

async function fetchRepos(provider: string) {
  loading.value = true;
  error.value = "";
  try {
    const list = await api.get(`/projects/import/${provider}/repos/`);
    if (provider === "github") {
      githubRepos.value = list || [];
    } else {
      gitlabRepos.value = list || [];
    }
  } catch (e: any) {
    error.value = `Failed to fetch repositories: ${e.message}`;
  } finally {
    loading.value = false;
  }
}

async function createBlank() {
  if (!blankName.value) return;
  loading.value = true;
  error.value = "";
  try {
    const repo = await api.post("/projects/", {
      name: blankName.value,
      description: blankDesc.value,
      visibility: blankVis.value,
      owner_type: selectedOwner.value.type,
      owner_id: selectedOwner.value.id
    });
    router.push(`/${repo.path}`);
  } catch (e: any) {
    error.value = e.message || "An error occurred during project creation.";
  } finally {
    loading.value = false;
  }
}

function selectImportTarget(provider: string, repo: any) {
  importProvider.value = provider;
  importTarget.value = repo;
  importLocalName.value = repo.name;
  importDescription.value = repo.description || "";
  importVisibility.value = repo.private ? "private" : "public";
  showImportModal.value = true;
  error.value = "";
}

async function executeImport() {
  loading.value = true;
  error.value = "";
  try {
    const repo = await api.post("/projects/import/", {
      provider: importProvider.value,
      repo_name: importTarget.value.full_name,
      name: importLocalName.value,
      description: importDescription.value,
      visibility: importVisibility.value,
      owner_type: selectedOwner.value.type,
      owner_id: selectedOwner.value.id
    });
    showImportModal.value = false;
    router.push(`/${repo.path}`);
  } catch (e: any) {
    error.value = e.message || "Import failed. Please check repository name and try again.";
  } finally {
    loading.value = false;
  }
}

async function createFromUrl() {
  if (!urlCloneUrl.value || !urlName.value) return;
  loading.value = true;
  error.value = "";
  
  let formattedUrl = urlCloneUrl.value;
  if (urlUsername.value || urlToken.value) {
    try {
      const urlObj = new URL(urlCloneUrl.value);
      const userPart = urlUsername.value ? encodeURIComponent(urlUsername.value) : "";
      const passPart = urlToken.value ? encodeURIComponent(urlToken.value) : "";
      
      const authPart = userPart && passPart ? `${userPart}:${passPart}` : (userPart || passPart);
      urlObj.username = authPart;
      formattedUrl = urlObj.toString();
    } catch {
      // Fallback
    }
  }

  try {
    const repo = await api.post("/projects/import/", {
      provider: "custom",
      clone_url: formattedUrl,
      name: urlName.value,
      description: urlDesc.value,
      visibility: urlVis.value,
      owner_type: selectedOwner.value.type,
      owner_id: selectedOwner.value.id
    });
    router.push(`/${repo.path}`);
  } catch (e: any) {
    error.value = e.message || "Failed to import from URL.";
  } finally {
    loading.value = false;
  }
}
</script>
