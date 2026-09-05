import { defineStore } from "pinia";
import { ref } from "vue";

interface Toast {
  id: number;
  message: string;
  type: "success" | "error" | "info";
}

export const useNotificationStore = defineStore("notifications", () => {
  const toasts = ref<Toast[]>([]);
  let nextId = 0;

  function show(message: string, type: Toast["type"] = "info") {
    const id = nextId++;
    toasts.value.push({ id, message, type });
    setTimeout(() => {
      toasts.value = toasts.value.filter((t) => t.id !== id);
    }, 4000);
  }

  return { toasts, show };
});
