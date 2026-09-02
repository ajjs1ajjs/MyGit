package api

import (
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ajjs1ajjs/MyGit/internal/auth"
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
	var body importRequest
	if err := jsonDecode(r, &body); err != nil {
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
	// default_branch is later passed to git as an argument: only a valid ref
	// name can ever be stored here (blocks git option injection).
	if !git.ValidRefName(defBranch) {
		writeErr(w, http.StatusBadRequest, "Invalid default branch name")
		return
	}
	ownerID, _ := anyInt64(body.OwnerID)
	ownerType, ownerPath, ownerID, group, ok := a.resolveOwner(w, r, p, body.OwnerType, ownerID)
	if !ok {
		return
	}
	_ = group
	path := ownerPath + "/" + body.Name
	if existing, _ := a.Store.GetRepoByPath(path); existing != nil {
		writeErr(w, http.StatusBadRequest, "Repository already exists")
		return
	}
	// Defense in depth: the owner component must be a safe path segment.
	if !validRepoName(ownerPath) {
		writeErr(w, http.StatusBadRequest, "Invalid owner")
		return
	}
	repo := &storage.Repository{
		OwnerType: ownerType, OwnerID: ownerID, Name: body.Name, Path: path,
		Description: body.Description, Visibility: vis, DefaultBranch: defBranch,
	}
	// custom_disk_path: absolute physical directory, superuser only.
	targetDir := a.Git.RepoPath(ownerPath, body.Name)
	if body.CustomDiskPath != "" {
		if !p.IsSuper {
			writeErr(w, http.StatusForbidden, "Custom storage paths require admin access")
			return
		}
		// Bounded custom storage: the directory must live inside the
		// configured custom storage root so neither creation nor later
		// deletion can ever touch arbitrary locations on disk.
		dir, ok := a.allowedCustomDir(body.CustomDiskPath)
		if !ok {
			writeErr(w, http.StatusBadRequest, "Custom storage path must be an absolute directory inside MYGIT_CUSTOM_REPOS_ROOT")
			return
		}
		targetDir = dir
		repo.StoragePath = targetDir
	}
	id, err := a.Store.CreateRepo(repo)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if err := a.Git.InitBareAt(ownerPath, body.Name, defBranch, targetDir); err != nil {
		_ = a.Store.DeleteRepo(id)
		writeErr(w, http.StatusInternalServerError, "git init failed: "+err.Error())
		return
	}
	_ = a.Store.SetAccess(p.UserID, id, 50)
	a.Store.AddAuditEvent("repo.create", p.UserID, p.Username, "repository", repo.Path, "Repository "+repo.Path+" created")
	repo.ID = id
	writeJSON(w, http.StatusCreated, repoToMap(*repo))
}

func validRepoName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	// Reject dot-segments outright; they would resolve through filepath.Join.
	if name == "." || name == ".." || strings.HasPrefix(name, "..") {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

// safeRefArg reports whether ref may safely be passed to git as an argument.
// Accepts "HEAD", full/abbreviated hex SHAs and valid ref names; everything
// else — especially anything starting with "-" (git option injection, e.g.
// `git log --output=<path>` = arbitrary file write) — is rejected.
func safeRefArg(ref string) bool {
	if ref == "" {
		return false
	}
	if ref == "HEAD" {
		return true
	}
	if isHexSHA(ref) {
		return true
	}
	return git.ValidRefName(ref)
}

func isHexSHA(s string) bool {
	if len(s) < 4 || len(s) > 40 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// allowedCustomDir validates a superuser-provided absolute storage directory:
// it must live inside the configured custom storage root (MYGIT_CUSTOM_REPOS_ROOT,
// default: the base data dir) and must not be the root itself, so that neither
// project creation nor project deletion can ever touch arbitrary directories.
func (a *App) allowedCustomDir(raw string) (string, bool) {
	p := filepath.Clean(raw)
	if !filepath.IsAbs(p) {
		return "", false
	}
	root := filepath.Clean(a.Cfg.CustomReposRoot)
	if root == "" {
		return "", false
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	// Belt and braces: never treat a filesystem volume root as a repo dir.
	if p == "/" || (len(p) == 3 && p[1] == ':' && os.IsPathSeparator(p[2])) {
		return "", false
	}
	return p, true
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
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	fields := map[string]any{}
	if v, ok := body["description"].(string); ok {
		fields["description"] = v
	}
	if v, ok := body["visibility"].(string); ok {
		if v != "public" && v != "private" && v != "internal" {
			writeErr(w, http.StatusBadRequest, "visibility must be public, private or internal")
			return
		}
		fields["visibility"] = v
	}
	if v, ok := body["default_branch"].(string); ok {
		// See handleCreateProject: git receives this value as an argument.
		if v == "" || !git.ValidRefName(v) {
			writeErr(w, http.StatusBadRequest, "Invalid default branch name")
			return
		}
		fields["default_branch"] = v
	}
	if len(fields) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"detail": "updated"})
		return
	}
	if err := a.Store.UpdateRepo(id, fields); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	a.Store.AddAuditEvent("repo.update", p.UserID, p.Username, "repository", repo.Path, "Repository "+repo.Path+" updated")
	writeJSON(w, http.StatusOK, map[string]any{"detail": "updated"})
}

// repoPathParts splits a stored "owner/name" repo path and validates both
// components so a poisoned/legacy row can never reach filepath.Join.
func repoPathParts(path string) (string, string) {
	parts := strings.Split(path, "/")
	if len(parts) < 2 || !validRepoName(parts[0]) || !validRepoName(parts[1]) {
		return "", ""
	}
	return parts[0], parts[1]
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
	owner, name := repoPathParts(repo.Path)
	if owner == "" || name == "" {
		writeErr(w, http.StatusInternalServerError, "Invalid repository path")
		return
	}
	_ = a.Git.Remove(owner, name)
	if repo.StoragePath != "" {
		// Data-loss guard: only recurse into a custom directory that is
		// inside the configured custom storage root. A stray or legacy path
		// outside it is left on disk and reported to the log.
		if dir, ok := a.allowedCustomDir(repo.StoragePath); ok {
			_ = os.RemoveAll(dir)
		} else {
			log.Printf("repo %d: custom storage path %q is outside MYGIT_CUSTOM_REPOS_ROOT; directory left on disk", repo.ID, repo.StoragePath)
		}
	}
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
	owner, name := repoPathParts(repo.Path)
	if owner == "" || name == "" {
		writeErr(w, http.StatusInternalServerError, "Invalid repository path")
		return
	}
	// repoDir honors a custom storage path: forking a custom-path repo from
	// the standard location cloned a missing (or wrong) directory.
	srcDir := a.repoDir(repo)
	newPath := p.Username + "/" + repo.Name
	if !auth.ValidUsername(p.Username) {
		writeErr(w, http.StatusBadRequest, "Invalid owner")
		return
	}
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
	if repo.StoragePath != "" {
		return repo.StoragePath
	}
	owner, name := repoPathParts(repo.Path)
	if owner == "" || name == "" {
		return ""
	}
	return a.Git.RepoPath(owner, name)
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
	if !safeRefArg(ref) {
		writeErr(w, http.StatusBadRequest, "Invalid ref")
		return
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
	if !safeRefArg(ref) {
		writeErr(w, http.StatusBadRequest, "Invalid ref")
		return
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
	if dir == "" {
		writeErr(w, http.StatusInternalServerError, "Invalid repository path")
		return
	}
	sha := urlParam(r, "sha")
	ref := r.URL.Query().Get("ref")
	path := r.URL.Query().Get("path")

	var content []byte
	var err error
	if sha == "0" {
		// The frontend uses sha=0 as a sentinel meaning "resolve ref:path".
		if ref == "" {
			ref = repo.DefaultBranch
		}
		if !safeRefArg(ref) {
			writeErr(w, http.StatusBadRequest, "Invalid ref")
			return
		}
		content, err = a.Git.Blob(dir, ref, path)
	} else {
		if !safeRefArg(sha) {
			writeErr(w, http.StatusBadRequest, "Invalid sha")
			return
		}
		content, err = a.Git.BlobAtSHA(dir, sha)
	}
	if err != nil {
		writeErr(w, http.StatusNotFound, "Blob not found")
		return
	}
	// Text blobs are returned verbatim (encoding "text"); arbitrary binary
	// blobs are base64-encoded (encoding "base64") so they survive JSON
	// round-tripping intact. The frontend decodes based on the field.
	if utf8.Valid(content) && !strings.ContainsRune(string(content), '\x00') {
		writeJSON(w, http.StatusOK, map[string]any{
			"content": string(content), "sha": sha,
			"encoding": "text",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"content": base64.StdEncoding.EncodeToString(content), "sha": sha,
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
	if !safeRefArg(ref) {
		writeErr(w, http.StatusBadRequest, "Invalid ref")
		return
	}
	path := r.URL.Query().Get("path")
	lines, err := a.Git.Blame(dir, ref, path)
	if err != nil {
		writeErr(w, http.StatusNotFound, "Blame failed")
		return
	}
	if lines == nil {
		lines = []git.BlameLine{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": lines})
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
	if !safeRefArg(ref) {
		writeErr(w, http.StatusBadRequest, "Invalid ref")
		return
	}
	limit := 50
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n * 20
			if limit > 500 {
				limit = 500
			}
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
	if !safeRefArg(sha) {
		writeErr(w, http.StatusBadRequest, "Invalid sha")
		return
	}
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
	if !safeRefArg(sha) {
		writeErr(w, http.StatusBadRequest, "Invalid sha")
		return
	}
	diffs, err := a.Git.DiffFiles(dir, sha+"^", sha)
	if err != nil {
		// First commit in the repo has no parent; fall back to the empty tree.
		diffs, _ = a.Git.DiffFiles(dir, "", sha)
	}
	if diffs == nil {
		diffs = []git.FileDiff{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"diffs": diffs})
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
		branches = []git.Ref{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": branches})
}

// requireRepoWrite enforces a minimum role for mutating git operations.
func (a *App) requireRepoWrite(w http.ResponseWriter, r *http.Request, minRole int) *storage.Repository {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return nil
	}
	p := a.principal(r)
	if a.Store.EffectiveRole(p.UserID, repo.ID, repo.OwnerID, p.IsSuper, repo.Visibility) < minRole {
		writeErr(w, http.StatusForbidden, "Insufficient permissions")
		return nil
	}
	return repo
}

func (a *App) handleCreateBranch(w http.ResponseWriter, r *http.Request) {
	// Creating a branch mutates the repo — developer role or above.
	repo := a.requireRepoWrite(w, r, 30)
	if repo == nil {
		return
	}
	var body struct {
		Name string `json:"name"`
		Ref  string `json:"ref"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	dir := a.repoDir(repo)
	src := body.Ref
	if src == "" {
		src = repo.DefaultBranch
	}
	if err := a.Git.CreateBranch(dir, body.Name, src); err != nil {
		writeErr(w, http.StatusBadRequest, "Branch creation failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"detail": "branch created"})
}

func (a *App) handleDeleteBranch(w http.ResponseWriter, r *http.Request) {
	// Deleting a branch mutates the repo — developer role or above.
	repo := a.requireRepoWrite(w, r, 30)
	if repo == nil {
		return
	}
	dir := a.repoDir(repo)
	name := urlParam(r, "name")
	if err := a.Git.DeleteBranch(dir, name); err != nil {
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
		tags = []git.Ref{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": tags})
}
