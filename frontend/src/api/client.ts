const BASE = "/api/v1";

// The session lives in HttpOnly cookies set by the server, so there is nothing
// to keep in localStorage that an XSS payload could read.
async function refreshSession(): Promise<boolean> {
  try {
    const res = await fetch(`${BASE}/auth/refresh/`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
    });
    return res.ok;
  } catch {
    return false;
  }
}

async function request(path: string, options: RequestInit = {}) {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };
  // Double-submit CSRF: echo the server-issued cookie on state-changing calls
  // (the server also accepts same-origin Origin/Referer as fallback).
  if (options.method && options.method !== "GET" && options.method !== "HEAD") {
    const m = document.cookie.match(/(?:^|;\s*)mygit_csrf=([^;]+)/);
    if (m) headers["X-CSRF-Token"] = decodeURIComponent(m[1]);
  }
  let res = await fetch(`${BASE}${path}`, { ...options, headers, credentials: "include" });
  if (res.status === 401) {
    const refreshed = await refreshSession();
    if (refreshed) {
      res = await fetch(`${BASE}${path}`, { ...options, headers, credentials: "include" });
    }
  }
  if (res.status === 204) return null;
  const data = await res.json();
  if (!res.ok) throw new Error(data.detail || "Request failed");
  return data.results !== undefined ? data.results : data;
}

export const api = {
  get: (path: string) => request(path),
  post: (path: string, body?: unknown) =>
    request(path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  patch: (path: string, body: unknown) =>
    request(path, { method: "PATCH", body: JSON.stringify(body) }),
  put: (path: string, body: unknown) =>
    request(path, { method: "PUT", body: JSON.stringify(body) }),
  delete: (path: string) => request(path, { method: "DELETE" }),
};
