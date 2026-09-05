package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajjs1ajjs/MyGit/internal/auth"
)

// renameDirRetry renames a directory, retrying briefly: on Windows renaming
// a freshly written directory can transiently fail while antivirus or
// indexer handles are still open.
func renameDirRetry(oldDir, newDir string) error {
	var err error
	for i := 0; i < 30; i++ {
		if err = os.Rename(oldDir, newDir); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// handleListUsers returns all users. Superuser only.
func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	if !p.IsSuper {
		writeErr(w, http.StatusForbidden, "Admin access required")
		return
	}
	users, err := a.Store.ListUsers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, map[string]any{
			"id": u.ID, "username": u.Username, "email": u.Email,
			"full_name": u.FullName, "bio": u.Bio,
			"is_superuser": u.IsSuperuser == 1, "is_active": u.IsActive == 1,
			"date_joined": u.CreatedAt, "created_at": u.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetUserProfile returns a public profile for a username.
func (a *App) handleGetUserProfile(w http.ResponseWriter, r *http.Request) {
	username := urlParam(r, "username")
	u, err := a.Store.GetUserByUsername(username)
	if err != nil || u == nil {
		writeErr(w, http.StatusNotFound, "User not found")
		return
	}
	// Email is personal data: expose it only to the owner and superusers.
	out := map[string]any{
		"id": u.ID, "username": u.Username,
		"full_name": u.FullName, "bio": u.Bio,
		"date_joined": u.CreatedAt,
	}
	if p := a.principal(r); p != nil && (p.IsSuper || p.Username == u.Username) {
		out["email"] = u.Email
	}
	writeJSON(w, http.StatusOK, out)
}

// handlePatchUser edits a user (admin). Username, email, full_name,
// is_superuser, is_active and optionally a new password.
func (a *App) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	actor := a.principal(r)
	if !actor.IsSuper {
		writeErr(w, http.StatusForbidden, "Admin access required")
		return
	}
	username := urlParam(r, "username")
	u, err := a.Store.GetUserByUsername(username)
	if err != nil || u == nil {
		writeErr(w, http.StatusNotFound, "User not found")
		return
	}
	var body map[string]any
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	fields := map[string]any{}
	if v, ok := body["full_name"].(string); ok {
		fields["full_name"] = v
	}
	if v, ok := body["bio"].(string); ok {
		fields["bio"] = v
	}
	if v, ok := body["is_superuser"].(bool); ok {
		fields["is_superuser"] = b2i(v)
	}
	if v, ok := body["is_active"].(bool); ok {
		fields["is_active"] = b2i(v)
	}
	// Changing your own admin flag would lock everyone out — refuse to demote
	// the last superuser.
	if v, ok := fields["is_superuser"]; ok && v.(int) == 0 && u.ID == actor.UserID {
		writeErr(w, http.StatusBadRequest, "You cannot remove your own admin role")
		return
	}
	if v, ok := body["email"].(string); ok {
		email := strings.TrimSpace(v)
		if email != "" {
			if existing, _ := a.Store.GetUserByEmail(email); existing != nil && existing.ID != u.ID {
				writeErr(w, http.StatusBadRequest, "Email is already in use")
				return
			}
			fields["email"] = email
		}
	}
	newUsername := ""
	if v, ok := body["username"].(string); ok {
		name := auth.NormalizeUsername(v)
		if !auth.ValidUsername(name) {
			writeErr(w, http.StatusBadRequest, "Invalid username")
			return
		}
		if existing, _ := a.Store.GetUserByUsername(name); existing != nil && existing.ID != u.ID {
			writeErr(w, http.StatusBadRequest, "Username is already in use")
			return
		}
		if name != u.Username {
			newUsername = name
		}
		fields["username"] = name
	}
	if v, ok := body["password"].(string); ok && v != "" {
		if !auth.CheckPasswordPolicy(v) {
			writeErr(w, http.StatusBadRequest, "Password must be at least 8 characters")
			return
		}
		hash, err := auth.HashPassword(v)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Hashing failed")
			return
		}
		fields["password_hash"] = hash
		// Force the user to re-login: bump token_version.
		fields["token_version"] = int64(u.TokenVersion + 1)
	}
	if len(fields) == 0 {
		writeErr(w, http.StatusBadRequest, "Nothing to update")
		return
	}

	// Renaming a user must migrate their repositories: stored paths are
	// "username/name" and bare directories live under repos/<username>/.
	// Without this a rename silently orphaned every repo of that user.
	var renamedDirs [][2]string
	if newUsername != "" {
		repos, err := a.Store.ListAccessibleRepos(0, true)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Database error")
			return
		}
		var toRename []string // repo names
		for _, rp := range repos {
			if rp.OwnerType != "user" || rp.OwnerID != u.ID || rp.StoragePath != "" {
				continue // custom-path repos live outside repos/<owner>/ on disk
			}
			parts := strings.SplitN(rp.Path, "/", 2)
			if len(parts) != 2 || parts[0] != u.Username {
				continue
			}
			newDir := a.Git.RepoPath(newUsername, parts[1])
			if _, err := os.Stat(newDir); err == nil {
				writeErr(w, http.StatusBadRequest, "Target repository directory already exists: "+newUsername+"/"+parts[1])
				return
			}
			toRename = append(toRename, parts[1])
		}
		for _, name := range toRename {
			oldDir := a.Git.RepoPath(u.Username, name)
			newDir := a.Git.RepoPath(newUsername, name)
			// os.Rename does not create the target parent directory.
			if err := os.MkdirAll(filepath.Dir(newDir), 0o755); err != nil {
				writeErr(w, http.StatusInternalServerError, "Failed to prepare repository directory")
				return
			}
			if err := renameDirRetry(oldDir, newDir); err != nil {
				for _, done := range renamedDirs {
					_ = os.Rename(done[1], done[0])
				}
				writeErr(w, http.StatusInternalServerError, "Failed to move repository directory")
				return
			}
			renamedDirs = append(renamedDirs, [2]string{oldDir, newDir})
		}
		// drop the old owner directory if it is now empty
		_ = os.Remove(filepath.Join(a.Git.Root, u.Username))
	}

	if err := a.Store.UpdateUser(u.ID, fields); err != nil {
		for _, done := range renamedDirs {
			_ = os.Rename(done[1], done[0])
		}
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if newUsername != "" {
		if err := a.Store.RenameUserRepoPaths(u.ID, u.Username, newUsername); err != nil {
			for _, done := range renamedDirs {
				_ = os.Rename(done[1], done[0])
			}
			_ = a.Store.UpdateUser(u.ID, map[string]any{"username": u.Username})
			writeErr(w, http.StatusInternalServerError, "Database error")
			return
		}
		a.Store.AddAuditEvent("user.rename", actor.UserID, actor.Username, "user", u.Username, "User '"+u.Username+"' renamed to '"+newUsername+"'")
	}
	a.Store.AddAuditEvent("user.update", actor.UserID, actor.Username, "user", u.Username, "User "+u.Username+" updated")
	writeJSON(w, http.StatusOK, map[string]any{"detail": "updated"})
}

// --- integration tokens (self-service) ---

func (a *App) requireSelf(w http.ResponseWriter, r *http.Request) bool {
	p := a.principal(r)
	username := urlParam(r, "username")
	if p.Username != username && !p.IsSuper {
		writeErr(w, http.StatusForbidden, "You can only manage your own integrations")
		return false
	}
	return true
}

func (a *App) handleListIntegrationTokens(w http.ResponseWriter, r *http.Request) {
	if !a.requireSelf(w, r) {
		return
	}
	u, err := a.Store.GetUserByUsername(urlParam(r, "username"))
	if err != nil || u == nil {
		writeErr(w, http.StatusNotFound, "User not found")
		return
	}
	tokens, err := a.Store.ListIntegrationTokens(u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	out := make([]map[string]any, 0, len(tokens))
	for _, t := range tokens {
		plain, _ := decryptToken(a.Cfg.JWTSecret, t.TokenEncrypted)
		out = append(out, map[string]any{
			"provider":     t.Provider,
			"masked_token": maskToken(plain),
			"created_at":   t.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleSaveIntegrationToken(w http.ResponseWriter, r *http.Request) {
	if !a.requireSelf(w, r) {
		return
	}
	u, err := a.Store.GetUserByUsername(urlParam(r, "username"))
	if err != nil || u == nil {
		writeErr(w, http.StatusNotFound, "User not found")
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Token    string `json:"token"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	provider := strings.ToLower(strings.TrimSpace(body.Provider))
	if provider != "github" && provider != "gitlab" {
		writeErr(w, http.StatusBadRequest, "provider must be github or gitlab")
		return
	}
	if strings.TrimSpace(body.Token) == "" {
		writeErr(w, http.StatusBadRequest, "token is required")
		return
	}
	enc, err := encryptToken(a.Cfg.JWTSecret, strings.TrimSpace(body.Token))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Encryption failed")
		return
	}
	if err := a.Store.UpsertIntegrationToken(u.ID, provider, enc); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "saved", "provider": provider, "masked_token": maskToken(strings.TrimSpace(body.Token))})
}

func (a *App) handleDeleteIntegrationToken(w http.ResponseWriter, r *http.Request) {
	if !a.requireSelf(w, r) {
		return
	}
	u, err := a.Store.GetUserByUsername(urlParam(r, "username"))
	if err != nil || u == nil {
		writeErr(w, http.StatusNotFound, "User not found")
		return
	}
	provider := strings.ToLower(urlParam(r, "provider"))
	if err := a.Store.DeleteIntegrationToken(u.ID, provider); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "deleted"})
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
