import { createRouter, createWebHistory } from "vue-router";

const routes = [
  { path: "/", name: "home", component: () => import("../views/HomePage.vue") },
  { path: "/auth/login", name: "login", component: () => import("../views/LoginPage.vue") },
  { path: "/auth/register", name: "register", component: () => import("../views/RegisterPage.vue") },
  { path: "/:username", name: "user-profile", component: () => import("../views/UserProfilePage.vue") },
  {
    path: "/:username/:repo",
    name: "repo-detail",
    component: () => import("../views/RepoDetailPage.vue"),
  },
  {
    path: "/:username/:repo/-/tree/:ref(.*)?",
    name: "repo-tree",
    component: () => import("../views/RepoTreePage.vue"),
  },
  {
    path: "/:username/:repo/-/blob/:ref(.*)?",
    name: "repo-blob",
    component: () => import("../views/RepoBlobPage.vue"),
  },
  {
    path: "/:username/:repo/-/commits/:ref(.*)?",
    name: "repo-commits",
    component: () => import("../views/RepoCommitsPage.vue"),
  },
  {
    path: "/:username/:repo/-/commit/:sha",
    name: "repo-commit",
    component: () => import("../views/RepoCommitPage.vue"),
  },
  {
    path: "/:username/:repo/-/branches",
    name: "repo-branches",
    component: () => import("../views/RepoBranchesPage.vue"),
  },
  {
    path: "/:username/:repo/-/tags",
    name: "repo-tags",
    component: () => import("../views/RepoTagsPage.vue"),
  },
  {
    path: "/:username/:repo/-/issues",
    name: "repo-issues",
    component: () => import("../views/RepoIssuesPage.vue"),
  },
  {
    path: "/:username/:repo/-/issues/:num",
    name: "repo-issue",
    component: () => import("../views/RepoIssuePage.vue"),
  },
  {
    path: "/:username/:repo/-/issues/new",
    name: "repo-issue-new",
    component: () => import("../views/RepoIssueNewPage.vue"),
  },
  {
    path: "/:username/:repo/-/merge_requests",
    name: "repo-mr-list",
    component: () => import("../views/RepoMrListPage.vue"),
  },
  {
    path: "/:username/:repo/-/merge_requests/:num",
    name: "repo-mr-detail",
    component: () => import("../views/RepoMrDetailPage.vue"),
  },
  {
    path: "/:username/:repo/-/merge_requests/new",
    name: "repo-mr-new",
    component: () => import("../views/RepoMrNewPage.vue"),
  },
  {
    path: "/:username/:repo/-/settings",
    name: "repo-settings",
    component: () => import("../views/RepoSettingsPage.vue"),
  },
  { path: "/:username/:repo/-/wiki", name: "repo-wiki", component: () => import("../views/RepoWikiPage.vue") },
  { path: "/groups", name: "groups", component: () => import("../views/GroupsListPage.vue") },
  { path: "/groups/:id", name: "group-detail", component: () => import("../views/GroupDetailPage.vue") },
  { path: "/search", name: "search", component: () => import("../views/SearchPage.vue") },
];

export default createRouter({
  history: createWebHistory(),
  routes,
});
