package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

func (a *App) gitRepoFromPath(r *http.Request) *storage.Repository {
	owner := urlParam(r, "owner")
	name := strings.TrimSuffix(urlParam(r, "repo"), ".git")
	repo, _ := a.Store.GetRepoByPath(owner + "/" + name)
	return repo
}

func (a *App) gitRole(r *http.Request, repo *storage.Repository) int {
	if p, err := a.authenticate(r); err == nil {
		return a.Store.EffectiveRole(p.UserID, repo.ID, repo.OwnerID, p.IsSuper, repo.Visibility)
	}
	if repo.Visibility == "public" {
		return 10
	}
	return 0
}

func (a *App) handleGitInfoRefs(w http.ResponseWriter, r *http.Request) {
	repo := a.gitRepoFromPath(r)
	if repo == nil {
		writeErr(w, http.StatusNotFound, "repository not found")
		return
	}
	service := r.URL.Query().Get("service")
	if service != "git-upload-pack" && service != "git-receive-pack" {
		writeErr(w, http.StatusBadRequest, "unknown git service")
		return
	}
	role := a.gitRole(r, repo)
	if service == "git-receive-pack" && role < 30 {
		w.Header().Set("WWW-Authenticate", `Basic realm="mygit"`)
		writeErr(w, http.StatusUnauthorized, "write access requires authentication")
		return
	}
	if role < 10 {
		w.Header().Set("WWW-Authenticate", `Basic realm="mygit"`)
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	parts := strings.Split(repo.Path, "/")
	dir := a.Git.RepoPath(parts[0], parts[1])
	out, err := a.Git.InfoRefs(dir, service)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "git error: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/x-git-"+strings.TrimPrefix(service, "git-")+"-advertisement")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

func (a *App) handleGitRPC(w http.ResponseWriter, r *http.Request) {
	repo := a.gitRepoFromPath(r)
	if repo == nil {
		writeErr(w, http.StatusNotFound, "repository not found")
		return
	}
	service := "git-upload-pack"
	if strings.HasSuffix(r.URL.Path, "git-receive-pack") {
		service = "git-receive-pack"
	}
	role := a.gitRole(r, repo)
	if service == "git-receive-pack" && role < 30 {
		w.Header().Set("WWW-Authenticate", `Basic realm="mygit"`)
		writeErr(w, http.StatusUnauthorized, "write access requires authentication")
		return
	}
	if role < 10 {
		w.Header().Set("WWW-Authenticate", `Basic realm="mygit"`)
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 256<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read body failed")
		return
	}
	parts := strings.Split(repo.Path, "/")
	dir := a.Git.RepoPath(parts[0], parts[1])
	out, err := a.Git.RPC(dir, service, body)
	if err != nil {
		w.Header().Set("Content-Type", "application/x-git-"+strings.TrimPrefix(service, "git-")+"-result")
		_, _ = w.Write(out)
		return
	}
	w.Header().Set("Content-Type", "application/x-git-"+strings.TrimPrefix(service, "git-")+"-result")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

// --- internal (git hooks / SSH) ---

func (a *App) handlePreReceive(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	repoPath := r.URL.Query().Get("repo")
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		newSHA, ref := fields[1], fields[2]
		if newSHA == strings.Repeat("0", 40) {
			// deleting a ref
			if ref == "refs/heads/main" || ref == "refs/heads/master" {
				writeErr(w, http.StatusForbidden, "deleting the default branch is not allowed")
				return
			}
		}
	}
	_ = repoPath
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (a *App) handlePostReceive(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo")
	parts := strings.Split(repoPath, "/")
	if len(parts) >= 2 {
		dir := a.Git.RepoPath(parts[0], parts[1])
		size := a.Git.CountSize(dir)
		if repo, _ := a.Store.GetRepoByPath(repoPath); repo != nil {
			_ = a.Store.UpdateRepo(repo.ID, map[string]any{"size_kb": size, "updated_at": storage.Now()})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (a *App) handleCheckAccess(w http.ResponseWriter, r *http.Request) {
	var body struct {
		KeyID  int64  `json:"key_id"`
		Repo   string `json:"repo"`
		Action string `json:"action"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"allowed": true})
}

func (a *App) handleAuthorizedKeys(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	u, err := a.Store.GetUserByUsername(username)
	if err != nil || u == nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	keys, _ := a.Store.ListSSHKeys(u.ID)
	var sb strings.Builder
	for _, k := range keys {
		if k.IsActive == 1 && k.PublicKey != "" {
			sb.WriteString(k.PublicKey)
			sb.WriteString("\n")
		}
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(sb.String()))
}
