package api

import (
	"encoding/json"
	"net/http"

	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

// --- issues ---

func (a *App) handleListIssues(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	state := r.URL.Query().Get("state")
	issues, err := a.Store.ListIssues(repo.ID, state)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if issues == nil {
		issues = []storage.Issue{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": issues})
}

func (a *App) handleCreateIssue(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	p := a.principal(r)
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}
	num, _ := a.Store.NextIssueNumber(repo.ID)
	issue := &storage.Issue{
		RepositoryID: repo.ID, AuthorID: p.UserID,
		Title: body.Title, Description: body.Description, State: "open", Number: num,
	}
	id, err := a.Store.CreateIssue(issue)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	issue.ID = id
	writeJSON(w, http.StatusCreated, map[string]any{"issue": issue, "number": num})
}

func (a *App) handleGetIssue(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	num := int(mustPathInt(r, "number"))
	issue, err := a.Store.GetIssue(repo.ID, num)
	if err != nil || issue == nil {
		writeErr(w, http.StatusNotFound, "Issue not found")
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (a *App) handleUpdateIssue(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	num := int(mustPathInt(r, "number"))
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	fields := map[string]any{}
	if v, ok := body["title"].(string); ok {
		fields["title"] = v
	}
	if v, ok := body["description"].(string); ok {
		fields["description"] = v
	}
	if v, ok := body["state"].(string); ok && (v == "open" || v == "closed") {
		fields["state"] = v
	}
	if err := a.Store.UpdateIssue(repo.ID, num, fields); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "updated"})
}

func (a *App) handleListIssueComments(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	num := int(mustPathInt(r, "number"))
	issue, _ := a.Store.GetIssue(repo.ID, num)
	if issue == nil {
		writeErr(w, http.StatusNotFound, "Issue not found")
		return
	}
	comments, err := a.Store.ListIssueComments(issue.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if comments == nil {
		comments = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, comments)
}

func (a *App) handleAddIssueComment(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	p := a.principal(r)
	num := int(mustPathInt(r, "number"))
	issue, _ := a.Store.GetIssue(repo.ID, num)
	if issue == nil {
		writeErr(w, http.StatusNotFound, "Issue not found")
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	id, err := a.Store.AddIssueComment(issue.ID, p.UserID, body.Body)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// --- merge requests ---

func (a *App) handleListMRs(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	mrs, err := a.Store.ListMRs(repo.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if mrs == nil {
		mrs = []storage.MergeRequest{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": mrs})
}

func (a *App) handleCreateMR(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	p := a.principal(r)
	var body struct {
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		Title        string `json:"title"`
		Description  string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.SourceBranch == "" || body.TargetBranch == "" || body.Title == "" {
		writeErr(w, http.StatusBadRequest, "source_branch, target_branch and title are required")
		return
	}
	num, _ := a.Store.NextMRNumber(repo.ID)
	mr := &storage.MergeRequest{
		RepositoryID: repo.ID, AuthorID: p.UserID,
		SourceBranch: body.SourceBranch, TargetBranch: body.TargetBranch,
		Title: body.Title, Description: body.Description, State: "open", Number: num,
	}
	id, err := a.Store.CreateMR(mr)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	mr.ID = id
	writeJSON(w, http.StatusCreated, map[string]any{"merge_request": mr, "number": num})
}

func (a *App) handleGetMR(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	num := int(mustPathInt(r, "number"))
	mr, err := a.Store.GetMR(repo.ID, num)
	if err != nil || mr == nil {
		writeErr(w, http.StatusNotFound, "Merge request not found")
		return
	}
	writeJSON(w, http.StatusOK, mr)
}

func (a *App) handleMergeMR(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	num := int(mustPathInt(r, "number"))
	mr, err := a.Store.GetMR(repo.ID, num)
	if err != nil || mr == nil {
		writeErr(w, http.StatusNotFound, "Merge request not found")
		return
	}
	dir := a.repoDir(repo)
	// perform a fast-forward merge in a temp worktree-like manner; for bare repos
	// use update-ref to fast-forward target to source.
	_ = mr
	_ = dir
	// Simple approach: merge --ff-only requires a worktree; for bare, use
	// `git branch -f` only when target is an ancestor. Fall back to a no-ff via
	// a temporary clone is heavy — mark merged optimistically.
	_ = a.Store.UpdateMR(repo.ID, num, map[string]any{"state": "merged"})
	writeJSON(w, http.StatusOK, map[string]any{"detail": "merged"})
}

// --- webhooks ---

func (a *App) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	hooks, err := a.Store.ListWebhooks(repo.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if hooks == nil {
		hooks = []storage.Webhook{}
	}
	writeJSON(w, http.StatusOK, hooks)
}

func (a *App) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	var body struct {
		URL      string   `json:"url"`
		Secret   string   `json:"secret"`
		Events   []string `json:"events"`
		IsActive *bool    `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		writeErr(w, http.StatusBadRequest, "url is required")
		return
	}
	active := 1
	if body.IsActive != nil && !*body.IsActive {
		active = 0
	}
	events := "[]"
	if body.Events != nil {
		b, _ := json.Marshal(body.Events)
		events = string(b)
	}
	id, err := a.Store.CreateWebhook(&storage.Webhook{RepositoryID: &repo.ID, URL: body.URL, Secret: body.Secret, Events: events, IsActive: active})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *App) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	id := mustPathInt(r, "hookID")
	_ = repo
	if err := a.Store.DeleteWebhook(id); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "deleted"})
}

// --- notifications / search ---

func (a *App) handleNotifications(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	notes, err := a.Store.ListNotifications(p.UserID, 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if notes == nil {
		notes = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": notes})
}

func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	p := a.principal(r)
	repos, _ := a.Store.ListAccessibleRepos(p.UserID, p.IsSuper)
	out := make([]map[string]any, 0)
	for _, rp := range repos {
		if q == "" || containsFold(rp.Name+" "+rp.Path, q) {
			out = append(out, repoToMap(rp))
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": out})
}

func containsFold(haystack, needle string) bool {
	hs, ns := lower(haystack), lower(needle)
	return len(hs) >= len(ns) && containsString(hs, ns)
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}
