import { ref, watch } from "vue";
import { api } from "../api/client";

export function useApiGet<T = any>(pathFn: () => string) {
  const data = ref<T | null>(null);
  const loading = ref(false);
  const error = ref("");

  async function fetch() {
    loading.value = true;
    error.value = "";
    try {
      data.value = await api.get(pathFn());
    } catch (e: any) {
      error.value = e.message;
    }
    loading.value = false;
  }

  watch(pathFn, fetch, { immediate: true });

  return { data, loading, error, refresh: fetch };
}

export function useApiMutation<T = any>(method: "POST" | "PATCH" | "DELETE") {
  const loading = ref(false);
  const error = ref("");
  const result = ref<T | null>(null);

  async function execute(path: string, body?: any) {
    loading.value = true;
    error.value = "";
    try {
      if (method === "DELETE") {
        result.value = await api.delete(path);
      } else if (method === "PATCH") {
        result.value = await api.patch(path, body);
      } else {
        result.value = await api.post(path, body);
      }
    } catch (e: any) {
      error.value = e.message;
      throw e;
    }
    loading.value = false;
  }

  return { execute, loading, error, result };
}
