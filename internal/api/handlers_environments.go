package api

import (
	"net/http"
	"strings"

	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

// --- environments (per-repository deploy targets) ---
//
// Reads need repo read access (>=10); writes need writer role (>=30) via
// the shared requireRepoWrite helper.

func (a *App) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	envs, err := a.Store.ListEnvironments(repo.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if envs == nil {
		envs = []storage.Environment{}
	}
	writeJSON(w, http.StatusOK, envs)
}

func (a *App) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoWrite(w, r, 30)
	if repo == nil {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	id, err := a.Store.CreateEnvironment(repo.ID, strings.TrimSpace(body.Name))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": strings.TrimSpace(body.Name)})
}

func (a *App) handleDeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoWrite(w, r, 30)
	if repo == nil {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := a.Store.DeleteEnvironment(repo.ID, strings.TrimSpace(body.Name)); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "deleted"})
}

func (a *App) lookupEnv(w http.ResponseWriter, r *http.Request, repo *storage.Repository) *storage.Environment {
	name := strings.TrimSpace(r.URL.Query().Get("env"))
	if name == "" {
		var body struct {
			Env string `json:"env"`
		}
		_ = jsonDecode(r, &body)
		name = strings.TrimSpace(body.Env)
	}
	if name == "" {
		writeErr(w, http.StatusBadRequest, "env is required")
		return nil
	}
	env, err := a.Store.GetEnvironment(repo.ID, name)
	if err != nil || env == nil {
		writeErr(w, http.StatusNotFound, "Environment not found")
		return nil
	}
	return env
}

// --- variables (secrets encrypted at rest via the integration-token cipher) ---

func (a *App) handleListEnvVars(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	env := a.lookupEnv(w, r, repo)
	if env == nil {
		return
	}
	vars, err := a.Store.ListEnvVars(env.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if vars == nil {
		vars = []storage.EnvVar{}
	}
	writeJSON(w, http.StatusOK, vars)
}

func (a *App) handleSetEnvVar(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoWrite(w, r, 30)
	if repo == nil {
		return
	}
	env := a.lookupEnv(w, r, repo)
	if env == nil {
		return
	}
	var body struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Secret bool   `json:"secret"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	value := body.Value
	if body.Secret {
		enc, err := encryptToken(a.Cfg.JWTSecret, value)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Encryption failed")
			return
		}
		value = enc
	}
	if err := a.Store.SetEnvVar(env.ID, strings.TrimSpace(body.Name), value, body.Secret); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "saved"})
}

// --- deployments ---

func (a *App) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	env := a.lookupEnv(w, r, repo)
	if env == nil {
		return
	}
	deps, err := a.Store.ListDeployments(env.ID, 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if deps == nil {
		deps = []storage.Deployment{}
	}
	writeJSON(w, http.StatusOK, deps)
}

func (a *App) handleCreateDeployment(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoWrite(w, r, 30)
	if repo == nil {
		return
	}
	env := a.lookupEnv(w, r, repo)
	if env == nil {
		return
	}
	var body struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(body.Ref) == "" || strings.TrimSpace(body.SHA) == "" {
		writeErr(w, http.StatusBadRequest, "ref and sha are required")
		return
	}
	id, err := a.Store.CreateDeployment(repo.ID, env.ID, strings.TrimSpace(body.Ref), strings.TrimSpace(body.SHA))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *App) handleFinishDeployment(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoWrite(w, r, 30)
	if repo == nil {
		return
	}
	env := a.lookupEnv(w, r, repo)
	if env == nil {
		return
	}
	id := mustPathInt(r, "depID")
	var body struct {
		Status string `json:"status"`
		Log    string `json:"log"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	// Ensure the deployment belongs to this environment (no cross-env writes).
	deps, err := a.Store.ListDeployments(env.ID, 200)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	found := false
	for _, d := range deps {
		if d.ID == id {
			found = true
			break
		}
	}
	if !found {
		writeErr(w, http.StatusNotFound, "Deployment not found")
		return
	}
	if err := a.Store.FinishDeployment(id, body.Status, body.Log); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "recorded"})
}

