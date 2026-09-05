import { ref, onMounted } from "vue";
import { api } from "../api/client";

export function useRepo(username: string, repoName: string) {
  const repo = ref<any>(null);
  const repoId = ref("");
  const loading = ref(true);
  const error = ref("");

  async function fetch() {
    loading.value = true;
    try {
      const path = `${username}/${repoName}`;
      const found = await api.get(`/projects/by-path/${path}/`);
      if (found) {
        repo.value = found;
        repoId.value = found.id;
      } else {
        error.value = "Repository not found";
      }
    } catch (e: any) {
      error.value = e.message;
    } finally {
      loading.value = false;
    }
  }

  onMounted(fetch);

  return { repo, repoId, loading, error, refresh: fetch };
}
