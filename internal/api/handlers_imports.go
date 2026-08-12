package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

// providerClient fetches the repository list from GitHub/GitLab using the
// user's stored integration token.
func (a *App) providerRepos(p *principal, provider string) ([]map[string]any, error) {
	tok, err := a.Store.GetIntegrationToken(p.UserID, provider)
	if err != nil || tok == nil {
		return nil, fmt.Errorf("no %s integration token configured", provider)
	}
	token, err := decryptToken(a.Cfg.JWTSecret, tok.TokenEncrypted)
	if err != nil || token == "" {
		return nil, fmt.Errorf("could not decrypt %s integration token", provider)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 30 * time.Second}

	var apiURL string
	var req *http.Request
	if provider == "github" {
		apiURL = "https://api.github.com/user/repos?per_page=100&affiliation=owner,collaborator,organization_member"
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "MyGit")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("github api: %d", resp.StatusCode)
		}
		var items []struct {
			FullName    string `json:"full_name"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Private     bool   `json:"private"`
		}
		if err := json.Unmarshal(body, &items); err != nil {
			return nil, err
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"full_name": it.FullName, "name": it.Name,
				"description": it.Description, "private": it.Private,
			})
		}
		return out, nil
	}
	// GitLab
	apiURL = "https://gitlab.com/api/v4/projects?membership=true&per_page=100&simple=true"
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("PRIVATE-TOKEN", token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gitlab api: %d", resp.StatusCode)
	}
	var items []struct {
		PathWithNamespace string `json:"path_with_namespace"`
		Name              string `json:"name"`
		Description       string `json:"description"`
		Visibility        string `json:"visibility"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"full_name": it.PathWithNamespace, "name": it.Name,
			"description": it.Description,
			"private":     it.Visibility != "public",
		})
	}
	return out, nil
}

func (a *App) handleListProviderRepos(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	provider := strings.ToLower(urlParam(r, "provider"))
	if provider != "github" && provider != "gitlab" {
		writeErr(w, http.StatusBadRequest, "provider must be github or gitlab")
		return
	}
	repos, err := a.providerRepos(p, provider)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if repos == nil {
		repos = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, repos)
}

// importRequest is the shared body for /projects/import/ and the extended
// /projects/ blank-project creation.
type importRequest struct {
	Provider       string `json:"provider"` // github | gitlab | custom
	RepoName       string `json:"repo_name"`
	CloneURL       string `json:"clone_url"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Visibility     string `json:"visibility"`
	OwnerType      string `json:"owner_type"`
	OwnerID        any    `json:"owner_id"`
	CustomDiskPath string `json:"custom_disk_path"`
	DefaultBranch  string `json:"default_branch"`
}

// anyInt64 coerces a JSON-decoded owner_id (number, string or json.Number)
// into an int64. The SPA sends the id as a string.
func anyInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case json.Number:
		n, err := t.Int64()
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n, err == nil
	case nil:
		return 0, true
	}
	return 0, false
}

func (a *App) handleImportProject(w http.ResponseWriter, r *http.Request) {
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
	provider := strings.ToLower(body.Provider)
	if provider != "github" && provider != "gitlab" && provider != "custom" {
		writeErr(w, http.StatusBadRequest, "provider must be github, gitlab or custom")
		return
	}

	// Resolve the target owner (user or group) and on-disk destination.
	ownerID, _ := anyInt64(body.OwnerID)
	ownerType, ownerPath, ownerID, group, ok := a.resolveOwner(w, r, p, body.OwnerType, ownerID)
	if !ok {
		return
	}
	_ = group

	repo := &storage.Repository{
		OwnerType: ownerType, OwnerID: ownerID, Name: body.Name,
		Path:          ownerPath + "/" + body.Name,
		Description:   body.Description,
		Visibility:    body.Visibility,
		DefaultBranch: body.DefaultBranch,
	}
	if repo.Visibility != "public" && repo.Visibility != "private" && repo.Visibility != "internal" {
		repo.Visibility = "private"
	}
	if repo.DefaultBranch == "" {
		repo.DefaultBranch = "main"
	}
	if existing, _ := a.Store.GetRepoByPath(repo.Path); existing != nil {
		writeErr(w, http.StatusBadRequest, "A repository with this name already exists")
		return
	}

	// custom_disk_path: absolute physical directory, superuser only.
	targetDir := ""
	if body.CustomDiskPath != "" {
		if !p.IsSuper {
			writeErr(w, http.StatusForbidden, "Custom storage paths require admin access")
			return
		}
		targetDir = filepath.Clean(body.CustomDiskPath)
		if !filepath.IsAbs(targetDir) {
			writeErr(w, http.StatusBadRequest, "Custom storage path must be absolute")
			return
		}
		repo.StoragePath = targetDir
	} else {
		targetDir = a.Git.RepoPath(ownerPath, body.Name)
	}

	// Resolve the clone URL and run the import.
	cloneURL, err := a.resolveCloneURL(p, provider, body, targetDir)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	jobID, err := a.Store.CreateImportJob(provider, repo.Path)
	if err != nil {
		jobID = 0
	}

	id, err := a.Store.CreateRepo(repo)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if err := a.Git.ImportBare(cloneURL, targetDir); err != nil {
		_ = a.Store.DeleteRepo(id)
		if jobID > 0 {
			a.Store.FinishImportJob(jobID, "failed", err.Error())
		}
		writeErr(w, http.StatusBadGateway, "Import failed: "+err.Error())
		return
	}
	_ = a.Store.SetAccess(p.UserID, id, 50)
	if ownerType == "organization" && ownerID > 0 {
		// Group owners keep access.
		_ = a.Store.SetAccess(ownerID, id, 50)
	}
	repo.ID = id
	a.Store.AddAuditEvent("repo.import", p.UserID, p.Username, "repository", repo.Path, "Repository "+repo.Path+" imported")
	if jobID > 0 {
		a.Store.FinishImportJob(jobID, "success", "")
	}
	writeJSON(w, http.StatusCreated, repoToMap(*repo))
}

// resolveOwner maps owner_type/owner_id to the on-disk owner path.
func (a *App) resolveOwner(w http.ResponseWriter, r *http.Request, p *principal, ownerType string, ownerID int64) (string, string, int64, *storage.Group, bool) {
	if ownerType == "organization" && ownerID > 0 {
		g, err := a.Store.GetGroup(ownerID)
		if err != nil || g == nil {
			writeErr(w, http.StatusNotFound, "Group not found")
			return "", "", 0, nil, false
		}
		if !p.IsSuper && a.Store.GroupRole(p.UserID, g.ID) < 50 {
			writeErr(w, http.StatusForbidden, "Group owner access required")
			return "", "", 0, nil, false
		}
		return "organization", g.Path, g.ID, g, true
	}
	// user-owned: must be self
	if ownerType != "" && ownerType != "user" {
		writeErr(w, http.StatusBadRequest, "Invalid owner_type")
		return "", "", 0, nil, false
	}
	if ownerID != 0 && ownerID != p.UserID {
		writeErr(w, http.StatusForbidden, "You can only create projects in your own namespace")
		return "", "", 0, nil, false
	}
	return "user", p.Username, p.UserID, nil, true
}

// resolveCloneURL builds the URL git will clone from.
func (a *App) resolveCloneURL(p *principal, provider string, body importRequest, targetDir string) (string, error) {
	switch provider {
	case "github":
		tok, _ := a.Store.GetIntegrationToken(p.UserID, "github")
		if tok == nil {
			return "", fmt.Errorf("no github integration configured")
		}
		token, _ := decryptToken(a.Cfg.JWTSecret, tok.TokenEncrypted)
		if body.RepoName == "" {
			return "", fmt.Errorf("repo_name is required")
		}
		return "https://x-access-token:" + url.QueryEscape(token) + "@github.com/" + body.RepoName + ".git", nil
	case "gitlab":
		tok, _ := a.Store.GetIntegrationToken(p.UserID, "gitlab")
		if tok == nil {
			return "", fmt.Errorf("no gitlab integration configured")
		}
		token, _ := decryptToken(a.Cfg.JWTSecret, tok.TokenEncrypted)
		if body.RepoName == "" {
			return "", fmt.Errorf("repo_name is required")
		}
		return "https://oauth2:" + url.QueryEscape(token) + "@gitlab.com/" + body.RepoName + ".git", nil
	default: // custom
		u, err := url.Parse(body.CloneURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return "", fmt.Errorf("clone_url must be a valid http(s) URL")
		}
		return body.CloneURL, nil
	}
}

// --- server disk browser (admin) ---

func (a *App) handleBrowseDisk(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	current, parent, dirs, err := listDirectories(path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if dirs == nil {
		dirs = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"current_path": current,
		"parent_path":  parent,
		"directories":  dirs,
	})
}

func listDirectories(path string) (current, parent string, dirs []string, err error) {
	if path == "" {
		// Top level: on Unix "/", on Windows the drive letters.
		if os.PathSeparator == '/' {
			current = "/"
			parent = ""
		} else {
			current = ""
			parent = ""
		}
		return current, parent, rootDirs(), nil
	}
	current = filepath.Clean(path)
	if !filepath.IsAbs(current) {
		return "", "", nil, fmt.Errorf("path must be absolute")
	}
	info, err := os.Stat(current)
	if err != nil {
		return "", "", nil, fmt.Errorf("directory does not exist")
	}
	if !info.IsDir() {
		return "", "", nil, fmt.Errorf("not a directory")
	}
	if parentPath := filepath.Dir(current); parentPath != current {
		parent = parentPath
	}
	entries, err := os.ReadDir(current)
	if err != nil {
		return "", "", nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return current, parent, dirs, nil
}

func rootDirs() []string {
	if os.PathSeparator == '/' {
		dirs := []string{}
		entries, err := os.ReadDir("/")
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					dirs = append(dirs, "/"+e.Name())
				}
			}
		}
		return dirs
	}
	// Windows: logical drives.
	var dirs []string
	for _, d := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		p := string(d) + ":\\"
		if _, err := os.Stat(p); err == nil {
			dirs = append(dirs, p)
		}
	}
	return dirs
}

func (a *App) handleCreateDiskFolder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ParentPath string `json:"parent_path"`
		Name       string `json:"name"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		writeErr(w, http.StatusBadRequest, "Invalid folder name")
		return
	}
	parent := filepath.Clean(body.ParentPath)
	if !filepath.IsAbs(parent) {
		writeErr(w, http.StatusBadRequest, "parent_path must be absolute")
		return
	}
	if err := os.MkdirAll(filepath.Join(parent, name), 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "Failed to create folder")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "created", "path": filepath.Join(parent, name)})
}
