package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"strings"

	"github.com/ajjs1ajjs/MyGit/internal/auth"
	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	username := auth.NormalizeUsername(body.Username)
	if username == "" || body.Password == "" {
		writeErr(w, http.StatusBadRequest, "username and password are required")
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
		writeErr(w, http.StatusBadRequest, "Username already exists")
		return
	}
	if existing, _ := a.Store.GetUserByEmail(email); existing != nil {
		writeErr(w, http.StatusBadRequest, "Email already registered")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Hashing failed")
		return
	}
	id, err := a.Store.CreateUser(&storage.User{
		Username: username, Email: email, PasswordHash: hash,
		FullName: username, IsActive: 1, IsSuperuser: 1,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	access, refresh, _ := a.Auth.TokenPair(id, username)
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
	access, refresh, _ := a.Auth.TokenPair(u.ID, u.Username)
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	claims, err := a.Auth.Parse(body.Refresh, "refresh")
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}
	access, refresh, _ := a.Auth.TokenPair(claims.UserID, claims.Username)
	writeJSON(w, http.StatusOK, map[string]any{"access": access, "refresh": refresh})
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	raw := "mygit_pat_" + auth.RandomToken(32)
	hash := auth.HashToken(raw)
	scopes := "[]"
	if body.Scopes != nil {
		b, _ := json.Marshal(body.Scopes)
		scopes = string(b)
	}
	id, err := a.Store.CreateToken(p.UserID, body.Name, hash, scopes)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "token": raw, "name": body.Name})
}

func (a *App) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "tokenID")
	_ = p
	if err := a.Store.DeleteToken(id); err != nil {
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
