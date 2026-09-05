import { defineStore } from "pinia";
import { ref } from "vue";
import { api } from "../api/client";

interface User {
  id: string; username: string; email?: string;
  full_name?: string; bio?: string; avatar?: string;
  must_change_password?: boolean; is_superuser?: boolean;
}

export const useAuthStore = defineStore("auth", () => {
  const user = ref<User | null>(null);

  async function login(identity: string, password: string, otp?: string) {
    const data = await api.post("/auth/login/", { login: identity, password, otp });
    user.value = data.user;
    return data.user;
  }

  async function register(username: string, password: string) {
    const data = await api.post("/auth/register/", { username, password });
    user.value = data.user;
    return data.user;
  }

  async function fetchMe() {
    try { user.value = await api.get("/users/me/"); }
    catch { user.value = null; }
  }

  async function logout() {
    user.value = null;
    try { await api.post("/auth/logout/"); } catch { /* cookie may already be gone */ }
  }

  return { user, login, register, fetchMe, logout };
});
