package api

import (
	"net/http"
	"strings"

	"github.com/ajjs1ajjs/MyGit/internal/auth"
)

// --- runner registration (superuser) ---

func (a *App) handleRegisterRunner(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	var body struct {
		Name string `json:"name"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len(name) > 100 {
		writeErr(w, http.StatusBadRequest, "name is required (max 100 chars)")
		return
	}
	rawToken, err := auth.RandomToken(32)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Token generation failed")
		return
	}
	raw := "mygit_run_" + rawToken
	id, err := a.Store.RegisterRunner(name, auth.HashToken(raw))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	a.Store.AddAuditEvent("runner.registered", p.UserID, p.Username, "runner", name, "Runner '"+name+"' registered")
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": name, "token": raw})
}

func (a *App) handleListRunners(w http.ResponseWriter, r *http.Request) {
	runners, err := a.Store.ListRunners()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	out := make([]map[string]any, 0, len(runners))
	for _, rn := range runners {
		out = append(out, map[string]any{
			"id": rn.ID, "name": rn.Name, "is_active": rn.IsActive,
			"last_seen": rn.LastSeen, "created_at": rn.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleDeleteRunner(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "id")
	if err := a.Store.DeleteRunner(id); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	a.Store.AddAuditEvent("runner.deleted", p.UserID, p.Username, "runner", "", "Runner deleted")
	writeJSON(w, http.StatusOK, map[string]any{"detail": "deleted"})
}

// --- runner agent protocol (Bearer mygit_run_* token, no user session) ---

func (a *App) runnerAuth(r *http.Request) (int64, bool) {
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		return 0, false
	}
	token := strings.TrimPrefix(authz, "Bearer ")
	if !strings.HasPrefix(token, "mygit_run_") {
		return 0, false
	}
	rn, err := a.Store.GetRunnerByHash(auth.HashToken(token))
	if err != nil || rn == nil || rn.IsActive != 1 {
		return 0, false
	}
	a.Store.TouchRunner(rn.ID)
	return rn.ID, true
}

func (a *App) handleRunnerClaim(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.runnerAuth(r); !ok {
		writeErr(w, http.StatusUnauthorized, "Invalid runner token")
		return
	}
	job, err := a.Store.ClaimPipelineJob()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if job == nil {
		writeJSON(w, http.StatusOK, map[string]any{"job": nil})
		return
	}
	repo, _ := a.Store.GetRepoByID(job.RepositoryID)
	path := ""
	if repo != nil {
		path = repo.Path
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": map[string]any{
		"id": job.ID, "repository_id": job.RepositoryID, "repo": path,
		"ref": job.Ref, "sha": job.SHA, "status": job.Status,
	}})
}

func (a *App) handleRunnerFinish(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.runnerAuth(r); !ok {
		writeErr(w, http.StatusUnauthorized, "Invalid runner token")
		return
	}
	id := mustPathInt(r, "id")
	var body struct {
		Status string `json:"status"`
		Log    string `json:"log"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if len(body.Log) > 1<<20 {
		body.Log = body.Log[:1<<20]
	}
	if err := a.Store.FinishPipelineJob(id, body.Status, body.Log); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "recorded"})
}

// --- user-facing pipeline views (repo read access) ---

func (a *App) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	jobs, err := a.Store.ListPipelineJobs(repo.ID, 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}
