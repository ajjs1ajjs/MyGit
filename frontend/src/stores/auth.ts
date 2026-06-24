import { defineStore } from "pinia";
import { ref } from "vue";
import { api } from "../api/client";

interface User {
  id: string; username: string; email: string;
  full_name?: string; bio?: string; avatar?: string;
  must_change_password?: boolean;
}

export const useAuthStore = defineStore("auth", () => {
  const user = ref<User | null>(null);
  const token = ref(localStorage.getItem("access_token") || "");

  async function login(email: string, password: string) {
    const data = await api.post("/auth/login/", { email, password });
    token.value = data.access;
    localStorage.setItem("access_token", data.access);
    localStorage.setItem("refresh_token", data.refresh);
    user.value = data.user;
    return data.user;
  }

  async function register(username: string, email: string, password: string) {
    const data = await api.post("/auth/register/", { username, email, password });
    token.value = data.access;
    localStorage.setItem("access_token", data.access);
    localStorage.setItem("refresh_token", data.refresh);
    user.value = data.user;
    return data.user;
  }

  async function fetchMe() {
    try { user.value = await api.get("/users/me/"); }
    catch { user.value = null; }
  }

  function logout() {
    user.value = null; token.value = "";
    localStorage.removeItem("access_token"); localStorage.removeItem("refresh_token");
  }

  return { user, token, login, register, fetchMe, logout };
});
