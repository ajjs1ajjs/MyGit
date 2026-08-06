package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ajjs1ajjs/MyGit/internal/git"
	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

func (a *App) handleListProjects(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	repos, err := a.Store.ListAccessibleRepos(p.UserID, p.IsSuper)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if repos == nil {
		repos = []storage.Repository{}
	}
	out := make([]map[string]any, 0, len(repos))
	for _, rp := range repos {
		out = append(out, repoToMap(rp))
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out, "count": len(out)})
}

func repoToMap(r storage.Repository) map[string]any {
	return map[string]any{
		"id": r.ID, "name": r.Name, "path": r.Path,
		"description": r.Description, "visibility": r.Visibility,
		"default_branch": r.DefaultBranch, "is_archived": r.IsArchived == 1,
		"owner_type": r.OwnerType, "owner_id": r.OwnerID,
		"size_kb": r.SizeKB, "created_at": r.CreatedAt, "updated_at": r.UpdatedAt,
	}
}

func (a *App) repoFromPath(w http.ResponseWriter, r *http.Request) (*storage.Repository, *principal) {
	p := a.principal(r) // may be nil for anonymous access to public repos
	owner := urlParam(r, "owner")
	name := strings.TrimSuffix(urlParam(r, "repo"), ".git")
	repo, err := a.Store.GetRepoByPath(owner + "/" + name)
	if err != nil || repo == nil {
		writeErr(w, http.StatusNotFound, "Repository not found")
		return nil, nil
	}
	return repo, p
}

func (a *App) handleProjectByPath(w http.ResponseWriter, r *http.Request) {
	repo, p := a.repoFromPath(w, r)
	if repo == nil {
		return
	}
	if p == nil {
		p, _ = a.authenticate(r) // optional auth: token may be present
	}
	uid := int64(0)
	isSuper := false
	if p != nil {
		uid = p.UserID
		isSuper = p.IsSuper
	}
	role := a.Store.EffectiveRole(uid, repo.ID, repo.OwnerID, isSuper, repo.Visibility)
	if role < 10 {
		writeErr(w, http.StatusForbidden, "Access denied")
		return
	}
	writeJSON(w, http.StatusOK, repoToMap(*repo))
}

func (a *App) handleGetProject(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "id")
	repo, err := a.Store.GetRepoByID(id)
	if err != nil || repo == nil {
		writeErr(w, http.StatusNotFound, "Repository not found")
		return
	}
	role := a.Store.EffectiveRole(p.UserID, repo.ID, repo.OwnerID, p.IsSuper, repo.Visibility)
	if role < 10 {
		writeErr(w, http.StatusForbidden, "Access denied")
		return
	}
	writeJSON(w, http.StatusOK, repoToMap(*repo))
}

func (a *App) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	var body struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		Visibility    string `json:"visibility"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if !validRepoName(body.Name) {
		writeErr(w, http.StatusBadRequest, "Invalid repository name")
		return
	}
	vis := body.Visibility
	if vis != "public" && vis != "private" && vis != "internal" {
		vis = "private"
	}
	defBranch := body.DefaultBranch
	if defBranch == "" {
		defBranch = "main"
	}
	path := p.Username + "/" + body.Name
	if existing, _ := a.Store.GetRepoByPath(path); existing != nil {
		writeErr(w, http.StatusBadRequest, "Repository already exists")
		return
	}
	repo := &storage.Repository{
		OwnerType: "user", OwnerID: p.UserID, Name: body.Name, Path: path,
		Description: body.Description, Visibility: vis, DefaultBranch: defBranch,
	}
	id, err := a.Store.CreateRepo(repo)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if err := a.Git.InitBare(p.Username, body.Name, defBranch); err != nil {
		_ = a.Store.DeleteRepo(id)
		writeErr(w, http.StatusInternalServerError, "git init failed: "+err.Error())
		return
	}
	_ = a.Store.SetAccess(p.UserID, id, 50)
	repo.ID = id
	writeJSON(w, http.StatusCreated, repoToMap(*repo))
}

func validRepoName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

func (a *App) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "id")
	repo, err := a.Store.GetRepoByID(id)
	if err != nil || repo == nil {
		writeErr(w, http.StatusNotFound, "Repository not found")
		return
	}
	if a.Store.EffectiveRole(p.UserID, repo.ID, repo.OwnerID, p.IsSuper, repo.Visibility) < 40 {
		writeErr(w, http.StatusForbidden, "Maintainer access required")
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	fields := map[string]any{}
	if v, ok := body["description"].(string); ok {
		fields["description"] = v
	}
	if v, ok := body["visibility"].(string); ok {
		fields["visibility"] = v
	}
	if v, ok := body["default_branch"].(string); ok {
		fields["default_branch"] = v
	}
	_ = a.Store.UpdateRepo(id, fields)
	writeJSON(w, http.StatusOK, map[string]any{"detail": "updated"})
}

func (a *App) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "id")
	repo, err := a.Store.GetRepoByID(id)
	if err != nil || repo == nil {
		writeErr(w, http.StatusNotFound, "Repository not found")
		return
	}
	if a.Store.EffectiveRole(p.UserID, repo.ID, repo.OwnerID, p.IsSuper, repo.Visibility) < 50 {
		writeErr(w, http.StatusForbidden, "Owner access required")
		return
	}
	_ = a.Git.Remove(strings.Split(repo.Path, "/")[0], strings.Split(repo.Path, "/")[1])
	_ = a.Store.DeleteRepo(id)
	writeJSON(w, http.StatusOK, map[string]any{"detail": "deleted"})
}

func (a *App) handleForkProject(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "id")
	repo, err := a.Store.GetRepoByID(id)
	if err != nil || repo == nil {
		writeErr(w, http.StatusNotFound, "Repository not found")
		return
	}
	parts := strings.Split(repo.Path, "/")
	srcDir := a.Git.RepoPath(parts[0], parts[1])
	newPath := p.Username + "/" + repo.Name
	if existing, _ := a.Store.GetRepoByPath(newPath); existing != nil {
		writeErr(w, http.StatusBadRequest, "A repository with this name already exists")
		return
	}
	fork := &storage.Repository{
		OwnerType: "user", OwnerID: p.UserID, Name: repo.Name, Path: newPath,
		Description: repo.Description, Visibility: "private", DefaultBranch: repo.DefaultBranch,
		IsFork: 1,
	}
	fid, err := a.Store.CreateRepo(fork)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if err := a.Git.Fork(p.Username, repo.Name, srcDir); err != nil {
		_ = a.Store.DeleteRepo(fid)
		writeErr(w, http.StatusInternalServerError, "fork failed: "+err.Error())
		return
	}
	_ = a.Store.SetAccess(p.UserID, fid, 50)
	fork.ID = fid
	writeJSON(w, http.StatusCreated, repoToMap(*fork))
}

// --- repo content ---

func (a *App) requireRepoAccess(w http.ResponseWriter, r *http.Request) *storage.Repository {
	p := a.principal(r)
	id := mustPathInt(r, "id")
	repo, err := a.Store.GetRepoByID(id)
	if err != nil || repo == nil {
		writeErr(w, http.StatusNotFound, "Repository not found")
		return nil
	}
	role := a.Store.EffectiveRole(p.UserID, repo.ID, repo.OwnerID, p.IsSuper, repo.Visibility)
	if role < 10 {
		writeErr(w, http.StatusForbidden, "Access denied")
		return nil
	}
	return repo
}

func (a *App) repoDir(repo *storage.Repository) string {
	parts := strings.Split(repo.Path, "/")
	return a.Git.RepoPath(parts[0], parts[1])
}

func (a *App) handleTree(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = repo.DefaultBranch
	}
	path := r.URL.Query().Get("path")
	recursive := r.URL.Query().Get("recursive") == "1" || r.URL.Query().Get("recursive") == "true"
	entries, err := a.Git.Tree(dir, ref, path, recursive)
	if err != nil {
		writeErr(w, http.StatusNotFound, "Path not found")
		return
	}
	if entries == nil {
		entries = []git.TreeEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (a *App) handleRaw(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = repo.DefaultBranch
	}
	path := r.URL.Query().Get("path")
	content, err := a.Git.Blob(dir, ref, path)
	if err != nil {
		writeErr(w, http.StatusNotFound, "File not found")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(content)
}

func (a *App) handleBlob(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	sha := urlParam(r, "sha")
	content, err := a.Git.BlobAtSHA(dir, sha)
	if err != nil {
		writeErr(w, http.StatusNotFound, "Blob not found")
		return
	}
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = repo.DefaultBranch
	}
	path := r.URL.Query().Get("path")
	_ = ref
	_ = path
	writeJSON(w, http.StatusOK, map[string]any{
		"content": string(content), "sha": sha,
		"encoding": "base64",
	})
}

func (a *App) handleBlame(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = repo.DefaultBranch
	}
	path := r.URL.Query().Get("path")
	out, err := a.Git.Blame(dir, ref, path)
	if err != nil {
		writeErr(w, http.StatusNotFound, "Blame failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"blame": out})
}

func (a *App) handleCommits(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = repo.DefaultBranch
	}
	limit := 50
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n * 20
		}
	}
	commits, err := a.Git.Commits(dir, ref, limit)
	if err != nil {
		writeErr(w, http.StatusNotFound, "No commits")
		return
	}
	if commits == nil {
		commits = []git.Commit{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": commits})
}

func (a *App) handleCommitDetail(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	sha := urlParam(r, "sha")
	commit, err := a.Git.CommitDetail(dir, sha)
	if err != nil {
		writeErr(w, http.StatusNotFound, "Commit not found")
		return
	}
	writeJSON(w, http.StatusOK, commit)
}

func (a *App) handleCommitDiff(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	sha := urlParam(r, "sha")
	diff, err := a.Git.Diff(dir, sha+"^", sha)
	if err != nil {
		diff, _ = a.Git.Diff(dir, "", sha)
	}
	writeJSON(w, http.StatusOK, map[string]any{"diff": diff})
}

func (a *App) handleBranches(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	branches, err := a.Git.Branches(dir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if branches == nil {
		branches = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": branches})
}

func (a *App) handleCreateBranch(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	var body struct {
		Name string `json:"name"`
		Ref  string `json:"ref"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	dir := a.repoDir(repo)
	src := body.Ref
	if src == "" {
		src = repo.DefaultBranch
	}
	if err := runGit(dir, "branch", body.Name, src); err != nil {
		writeErr(w, http.StatusBadRequest, "Branch creation failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"detail": "branch created"})
}

func (a *App) handleDeleteBranch(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	name := urlParam(r, "name")
	if err := runGit(dir, "branch", "-D", name); err != nil {
		writeErr(w, http.StatusBadRequest, "Branch deletion failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "branch deleted"})
}

func (a *App) handleTags(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	tags, err := a.Git.Tags(dir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if tags == nil {
		tags = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": tags})
}
