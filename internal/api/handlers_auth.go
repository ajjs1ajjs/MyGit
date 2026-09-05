package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ajjs1ajjs/MyGit/internal/auth"
	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

// Cookie names are namespaced by the listen port so that two MyGit instances
// on the same host (e.g. :8060 and :8061) never overwrite each other's
// session cookie (cookies are scoped per-domain, not per-port, and each
// instance has its own JWT secret).
func (a *App) accessCookie() string  { return fmt.Sprintf("mygit_access_%d", a.Cfg.Port) }
func (a *App) refreshCookie() string { return fmt.Sprintf("mygit_refresh_%d", a.Cfg.Port) }

// setAuthCookies stores the session in HttpOnly cookies so the SPA never has to
// keep JWTs in localStorage (XSS can no longer exfiltrate a long-lived token).
// SameSite=Strict blocks cross-site sending, mitigating CSRF. Secure is only
// enabled over TLS so plain-http dev keeps working.
func (a *App) setAuthCookies(w http.ResponseWriter, r *http.Request, access, refresh string) {
	secure := a.isHTTPS(r)
	http.SetCookie(w, &http.Cookie{Name: a.accessCookie(), Value: access, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: a.refreshCookie(), Value: refresh, Path: "/api/v1/auth", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
}

func (a *App) clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: a.accessCookie(), Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: a.refreshCookie(), Value: "", Path: "/api/v1/auth", HttpOnly: true, MaxAge: -1})
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	username := auth.NormalizeUsername(body.Username)
	if username == "" || body.Password == "" {
		writeErr(w, http.StatusBadRequest, "username and password are required")
		return
	}
	// Reject anything that could escape the repos root through filepath.Join
	// (.., /, \\, leading dot). See auth.ValidUsername.
	if !auth.ValidUsername(username) {
		writeErr(w, http.StatusBadRequest, "Username may only contain letters, digits, '_', '-' and '.' and must start with a letter or digit")
		return
	}
	if !auth.CheckPasswordPolicy(body.Password) {
		writeErr(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}
	email := strings.TrimSpace(body.Email)
	if email == "" {
		email = username + "@mygit.local"
	}
	if existing, _ := a.Store.GetUserByUsername(username); existing != nil {
		writeErr(w, http.StatusBadRequest, "Registration failed")
		return
	}
	if existing, _ := a.Store.GetUserByEmail(email); existing != nil {
		writeErr(w, http.StatusBadRequest, "Registration failed")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Hashing failed")
		return
	}
	// Only the first registered user becomes superuser (bootstrap); everyone
	// else registers as a regular user to preserve the authorization model.
	// The count+insert runs under a write lock, so concurrent first
	// registrations cannot both claim superuser.
	id, _, err := a.Store.RegisterUser(&storage.User{
		Username: username, Email: email, PasswordHash: hash,
		FullName: username, IsActive: 1,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	access, refresh, _ := a.Auth.TokenPair(id, username, 0)
	a.setAuthCookies(w, r, access, refresh)
	writeJSON(w, http.StatusCreated, map[string]any{
		"access": access, "refresh": refresh,
		"user": map[string]any{"id": id, "username": username, "email": body.Email},
	})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Login    string `json:"login"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	identity := body.Login
	if identity == "" {
		identity = body.Username
	}
	u, err := a.Store.GetUserByUsername(identity)
	if err != nil || u == nil {
		u, _ = a.Store.GetUserByEmail(identity)
	}
	if u == nil || !auth.VerifyPassword(u.PasswordHash, body.Password) {
		writeErr(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	if u.IsActive != 1 {
		writeErr(w, http.StatusForbidden, "Account is disabled")
		return
	}
	access, refresh, _ := a.Auth.TokenPair(u.ID, u.Username, int64(u.TokenVersion))
	a.setAuthCookies(w, r, access, refresh)
	writeJSON(w, http.StatusOK, map[string]any{
		"access": access, "refresh": refresh,
		"user": map[string]any{
			"id": u.ID, "username": u.Username, "email": u.Email,
			"is_superuser":         u.IsSuperuser == 1,
			"must_change_password": u.MustChangePassword == 1,
		},
	})
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Refresh string `json:"refresh"`
	}
	// The SPA sends an empty body and relies on the HttpOnly refresh cookie, so
	// a missing/empty body is fine — only programmatic clients send a token.
	_ = jsonDecode(r, &body)
	// Accept the refresh token from the HttpOnly cookie (SPA) or the request
	// body (programmatic clients).
	refreshToken := body.Refresh
	if refreshToken == "" {
		if c, err := r.Cookie(a.refreshCookie()); err == nil {
			refreshToken = c.Value
		}
	}
	if refreshToken == "" {
		writeErr(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}
	claims, err := a.Auth.Parse(refreshToken, "refresh")
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}
	// Fetch user from DB first to check current state.
	u, err := a.Store.GetUserByID(claims.UserID)
	if err != nil || u == nil {
		writeErr(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}
	// Re-check token version AFTER fetching user: this ensures that a password
	// change (which bumps token_version) immediately invalidates the refresh token,
	// even if there was a tiny race window between parsing and DB fetch.
	if claims.Ver != int64(u.TokenVersion) {
		writeErr(w, http.StatusUnauthorized, "Refresh token has been revoked")
		return
	}
	if u.IsActive != 1 {
		writeErr(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}
	access, refresh, _ := a.Auth.TokenPair(claims.UserID, claims.Username, int64(u.TokenVersion))
	a.setAuthCookies(w, r, access, refresh)
	writeJSON(w, http.StatusOK, map[string]any{"access": access, "refresh": refresh})
}

// handleLogout clears the session cookies. Server-side revocation of a stolen
// token is handled by token_version on password change; this endpoint clears
// the browser session.
func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.clearAuthCookies(w)
	writeJSON(w, http.StatusOK, map[string]any{"detail": "logged out"})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	u, _ := a.Store.GetUserByID(p.UserID)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "User not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": u.ID, "username": u.Username, "email": u.Email,
		"full_name": u.FullName, "bio": u.Bio,
		"is_superuser":         u.IsSuperuser == 1,
		"must_change_password": u.MustChangePassword == 1,
	})
}

func (a *App) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
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
	if err := a.Store.UpdateUser(p.UserID, fields); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "updated"})
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	u, _ := a.Store.GetUserByID(p.UserID)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "User not found")
		return
	}
	if u.MustChangePassword != 1 && !auth.VerifyPassword(u.PasswordHash, body.CurrentPassword) {
		writeErr(w, http.StatusBadRequest, "Invalid current password")
		return
	}
	if !auth.CheckPasswordPolicy(body.NewPassword) {
		writeErr(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}
	hash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Hashing failed")
		return
	}
	// Bumping token_version invalidates every previously issued access and
	// refresh token for this user, so a leaked token cannot outlive a password
	// change. Requires the column to have been read above; increment atomically.
	_, _ = a.Store.DB.Exec(`UPDATE users SET token_version = token_version + 1 WHERE id = ?`, u.ID)
	_ = a.Store.UpdateUser(u.ID, map[string]any{"password_hash": hash, "must_change_password": 0})
	writeJSON(w, http.StatusOK, map[string]any{"detail": "password changed"})
}

// --- ssh keys ---

func keyFingerprint(pubKey string) string {
	block, _ := pem.Decode([]byte(pubKey))
	if block != nil {
		pubKey = string(block.Bytes)
	}
	// ssh-rsa AAAA... format: fingerprint = sha256 of the base64-decoded key
	fields := strings.Fields(pubKey)
	if len(fields) < 2 {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "SHA256:" + base64.StdEncoding.EncodeToString(sum[:])
}

func (a *App) handleListKeys(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	keys, err := a.Store.ListSSHKeys(p.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if keys == nil {
		keys = []storage.SSHKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

func (a *App) handleAddKey(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	var body struct {
		Title     string `json:"title"`
		PublicKey string `json:"public_key"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(body.PublicKey) == "" {
		writeErr(w, http.StatusBadRequest, "public_key is required")
		return
	}
	id, err := a.Store.CreateSSHKey(p.UserID, body.Title, strings.TrimSpace(body.PublicKey), keyFingerprint(body.PublicKey))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *App) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "keyID")
	if err := a.Store.DeleteSSHKey(p.UserID, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "deleted"})
}

// --- personal access tokens ---

func (a *App) handleListTokens(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	tokens, err := a.Store.ListTokens(p.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	// strip hashes
	out := make([]map[string]any, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, map[string]any{
			"id": t.ID, "name": t.Name, "scopes": t.Scopes,
			"last_used_at": t.LastUsedAt, "expires_at": t.ExpiresAt, "created_at": t.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	var body struct {
		Name          string   `json:"name"`
		Scopes        []string `json:"scopes"`
		ExpiresInDays int      `json:"expires_in_days"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	rawToken, err := auth.RandomToken(32)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Token generation failed")
		return
	}
	raw := "mygit_pat_" + rawToken
	hash := auth.HashToken(raw)
	scopes := "[]"
	if body.Scopes != nil {
		b, _ := json.Marshal(body.Scopes)
		scopes = string(b)
	}
	expiresAt := ""
	if body.ExpiresInDays > 0 {
		expiresAt = time.Now().UTC().Add(time.Duration(body.ExpiresInDays) * 24 * time.Hour).Format("2006-01-02T15:04:05Z")
	}
	id, err := a.Store.CreateTokenWithExpiry(p.UserID, body.Name, hash, scopes, expiresAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "token": raw, "name": body.Name})
}

func (a *App) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "tokenID")
	if err := a.Store.DeleteToken(p.UserID, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "deleted"})
}

func mustPathInt(r *http.Request, name string) int64 {
	var id int64
	for _, c := range urlParam(r, name) {
		if c < '0' || c > '9' {
			return 0
		}
		id = id*10 + int64(c-'0')
	}
	return id
}
