import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "../stores/auth";

const RepoDetail = () => import("../views/RepoDetailPage.vue");

const routes = [
  { path: "/", name: "home", component: () => import("../views/HomePage.vue") },
  { path: "/auth/login", name: "login", component: () => import("../views/LoginPage.vue") },
  { path: "/auth/register", name: "register", component: () => import("../views/RegisterPage.vue") },
  { path: "/auth/change-password", name: "change-password", component: () => import("../views/ChangePasswordPage.vue") },
  { path: "/projects/new", name: "new-project", component: () => import("../views/NewProjectPage.vue") },
  { path: "/:username", name: "user-profile", component: () => import("../views/UserProfilePage.vue") },
  {
    path: "/:username/:repo",
    component: RepoDetail,
    children: [
      { path: "-/tree/:ref(.*)?", name: "repo-tree", component: () => import("../views/RepoTreePage.vue") },
      { path: "-/blob/:ref(.*)?", name: "repo-blob", component: () => import("../views/RepoBlobPage.vue") },
      { path: "-/commits/:ref(.*)?", name: "repo-commits", component: () => import("../views/RepoCommitsPage.vue") },
      { path: "-/commit/:sha", name: "repo-commit", component: () => import("../views/RepoCommitPage.vue") },
      { path: "-/branches", name: "repo-branches", component: () => import("../views/RepoBranchesPage.vue") },
      { path: "-/tags", name: "repo-tags", component: () => import("../views/RepoTagsPage.vue") },
      { path: "-/issues", name: "repo-issues", component: () => import("../views/RepoIssuesPage.vue") },
      { path: "-/issues/:num", name: "repo-issue", component: () => import("../views/RepoIssuePage.vue") },
      { path: "-/issues/new", name: "repo-issue-new", component: () => import("../views/RepoIssueNewPage.vue") },
      { path: "-/merge_requests", name: "repo-mr-list", component: () => import("../views/RepoMrListPage.vue") },
      { path: "-/merge_requests/:num", name: "repo-mr-detail", component: () => import("../views/RepoMrDetailPage.vue") },
      { path: "-/merge_requests/new", name: "repo-mr-new", component: () => import("../views/RepoMrNewPage.vue") },
      { path: "-/settings", name: "repo-settings", component: () => import("../views/RepoSettingsPage.vue") },
      { path: "-/wiki", name: "repo-wiki", component: () => import("../views/RepoWikiPage.vue") },
      { path: "-/blame/:ref(.*)?", name: "repo-blame", component: () => import("../views/RepoBlamePage.vue") },
    ],
  },
  { path: "/groups", name: "groups", component: () => import("../views/GroupsListPage.vue") },
  { path: "/groups/:id", name: "group-detail", component: () => import("../views/GroupDetailPage.vue") },
  { path: "/search", name: "search", component: () => import("../views/SearchPage.vue") },
  { path: "/manage/users", name: "admin-users", component: () => import("../views/AdminUsersPage.vue") },
  { path: "/manage/system", name: "admin-system", component: () => import("../views/AdminSystemPage.vue") },
];

const router = createRouter({ history: createWebHistory(), routes });

router.beforeEach(async (to, from) => {
  const auth = useAuthStore();

  // Sessions live in HttpOnly cookies (set by the server). If the store is
  // empty but a session cookie exists, hydrate it from /me.
  if (!auth.user) {
    await auth.fetchMe();
  }

  if (auth.user?.must_change_password && to.name !== "change-password") {
    return "/auth/change-password";
  }

  return true;
});

export default router;
