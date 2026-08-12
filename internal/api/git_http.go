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
	// Reject anything that isn't a plain owner/repo identifier before it can
	// reach filepath.Join below (blocks `..`, `%2F`, `.`, backslashes, etc.).
	if !validRepoName(owner) || !validRepoName(name) {
		return nil
	}
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
	if repo.StoragePath != "" {
		dir = repo.StoragePath
	}
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
	parts := strings.Split(repo.Path, "/")
	dir := a.Git.RepoPath(parts[0], parts[1])
	if repo.StoragePath != "" {
		dir = repo.StoragePath
	}
	// Stream the body straight into git's stdin (capped at 256MB) instead of
	// buffering it in memory — large pushes used to consume the whole payload.
	out, err := a.Git.RPC(dir, service, io.LimitReader(r.Body, 256<<20))
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
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (a *App) handlePostReceive(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo")
	parts := strings.Split(repoPath, "/")
	if len(parts) >= 2 {
		dir := a.Git.RepoPath(parts[0], parts[1])
		if repo, _ := a.Store.GetRepoByPath(repoPath); repo != nil {
			if repo.StoragePath != "" {
				dir = repo.StoragePath
			}
			size := a.Git.CountSize(dir)
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
	allowed, err := a.checkSSHAccess(body.KeyID, body.Repo, body.Action)
	if err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"allowed": allowed})
}

// checkSSHAccess resolves the key's owner and enforces the repo role for the
// requested action. read requires a visible repo (role >= 10); write (push)
// requires role >= 30.
func (a *App) checkSSHAccess(keyID int64, repoPath, action string) (bool, error) {
	if keyID <= 0 || repoPath == "" {
		return false, nil
	}
	key, err := a.Store.GetSSHKeyByID(keyID)
	if err != nil {
		return false, nil
	}
	if key == nil || key.IsActive != 1 {
		return false, nil
	}
	parts := strings.Split(strings.Trim(repoPath, "/"), "/")
	if len(parts) < 2 {
		return false, nil
	}
	owner, name := parts[0], parts[1]
	if !validRepoName(owner) || !validRepoName(name) {
		return false, nil
	}
	repo, err := a.Store.GetRepoByPath(owner + "/" + name)
	if err != nil || repo == nil {
		return false, nil
	}
	u, _ := a.Store.GetUserByID(key.UserID)
	if u == nil || u.IsActive != 1 {
		return false, nil
	}
	role := a.Store.EffectiveRole(u.ID, repo.ID, repo.OwnerID, u.IsSuperuser == 1, repo.Visibility)
	switch action {
	case "read", "clone", "pull":
		return role >= 10, nil
	case "write", "push":
		return role >= 30, nil
	}
	return false, nil
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
