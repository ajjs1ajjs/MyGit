package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/ajjs1ajjs/MyGit/internal/git"
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
	for i := range issues {
		issues[i].AuthorUsername = usernameOf(a.Store, issues[i].AuthorID)
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
	if err := jsonDecode(r, &body); err != nil || body.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}
	issue := &storage.Issue{
		RepositoryID: repo.ID, AuthorID: p.UserID, AuthorUsername: p.Username,
		Title: body.Title, Description: body.Description, State: "open",
	}
	id, num, err := a.Store.CreateIssue(issue)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	issue.ID = id
	issue.Number = num
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
	issue.AuthorUsername = usernameOf(a.Store, issue.AuthorID)
	writeJSON(w, http.StatusOK, issue)
}

func (a *App) handleUpdateIssue(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	num := int(mustPathInt(r, "number"))
	var body map[string]any
	if err := jsonDecode(r, &body); err != nil {
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
	out := make([]map[string]any, 0, len(comments))
	for _, c := range comments {
		var authorID int64
		if v, ok := c["author_id"].(int64); ok {
			authorID = v
		}
		m := make(map[string]any, len(c))
		for k, v := range c {
			m[k] = v
		}
		m["author_username"] = usernameOf(a.Store, authorID)
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
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
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	id, err := a.Store.AddIssueComment(issue.ID, p.UserID, body.Body)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "body": body.Body, "author_username": p.Username, "created_at": storage.Now()})
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
	for i := range mrs {
		mrs[i].AuthorUsername = usernameOf(a.Store, mrs[i].AuthorID)
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
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.SourceBranch == "" || body.TargetBranch == "" || body.Title == "" {
		writeErr(w, http.StatusBadRequest, "source_branch, target_branch and title are required")
		return
	}
	mr := &storage.MergeRequest{
		RepositoryID: repo.ID, AuthorID: p.UserID, AuthorUsername: p.Username,
		SourceBranch: body.SourceBranch, TargetBranch: body.TargetBranch,
		Title: body.Title, Description: body.Description, State: "open",
	}
	id, num, err := a.Store.CreateMR(mr)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	mr.ID = id
	mr.Number = num
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
	mr.AuthorUsername = usernameOf(a.Store, mr.AuthorID)
	writeJSON(w, http.StatusOK, mr)
}

func (a *App) handleListMRComments(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	num := int(mustPathInt(r, "number"))
	mr, _ := a.Store.GetMR(repo.ID, num)
	if mr == nil {
		writeErr(w, http.StatusNotFound, "Merge request not found")
		return
	}
	comments, err := a.Store.ListMRComments(mr.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	out := make([]map[string]any, 0, len(comments))
	for _, c := range comments {
		out = append(out, map[string]any{
			"id": c.ID, "body": c.Body, "created_at": c.CreatedAt,
			"author_username": usernameOf(a.Store, c.AuthorID),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleAddMRComment(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	p := a.principal(r)
	num := int(mustPathInt(r, "number"))
	mr, _ := a.Store.GetMR(repo.ID, num)
	if mr == nil {
		writeErr(w, http.StatusNotFound, "Merge request not found")
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	id, err := a.Store.AddMRComment(mr.ID, p.UserID, body.Body)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "author_username": p.Username})
}

func (a *App) handleMRDiff(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	num := int(mustPathInt(r, "number"))
	mr, _ := a.Store.GetMR(repo.ID, num)
	if mr == nil {
		writeErr(w, http.StatusNotFound, "Merge request not found")
		return
	}
	dir := a.repoDir(repo)
	// Compare the merge-base against the source branch so a stale target
	// doesn't produce a misleading diff.
	var diffs []git.FileDiff
	if base, err := a.Git.RefSHA(dir, mr.TargetBranch); err == nil {
		if mb, err := a.Git.RefSHA(dir, mr.SourceBranch); err == nil {
			diffs, _ = a.Git.DiffFiles(dir, base, mb)
		}
	}
	if diffs == nil {
		diffs = []git.FileDiff{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"diffs": diffs})
}

// --- wiki ---

func (a *App) handleListWiki(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	pages, err := a.Store.ListWiki(repo.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	out := make([]map[string]any, 0, len(pages))
	for _, p := range pages {
		out = append(out, map[string]any{
			"slug": p.Slug, "title": p.Title, "content": p.Content, "created_at": p.CreatedAt,
			"author_username": usernameOf(a.Store, p.AuthorID),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleCreateWiki(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoWrite(w, r, 30)
	if repo == nil {
		return
	}
	p := a.principal(r)
	var body struct {
		Slug    string `json:"slug"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := jsonDecode(r, &body); err != nil || strings.TrimSpace(body.Slug) == "" {
		writeErr(w, http.StatusBadRequest, "slug is required")
		return
	}
	slug := strings.TrimSpace(body.Slug)
	if strings.ContainsAny(slug, "/\\") || strings.HasPrefix(slug, ".") {
		writeErr(w, http.StatusBadRequest, "Invalid wiki slug")
		return
	}
	if err := a.Store.UpsertWiki(repo.ID, p.UserID, slug, body.Title, body.Content); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"slug": slug})
}

func (a *App) handleUpdateWiki(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoWrite(w, r, 30)
	if repo == nil {
		return
	}
	p := a.principal(r)
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	slug := urlParam(r, "slug")
	if err := a.Store.UpsertWiki(repo.ID, p.UserID, slug, body.Title, body.Content); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"slug": slug})
}

// usernameOf resolves a user ID to a username, returning "" when unknown.
func usernameOf(store *storage.Store, userID int64) string {
	if userID <= 0 {
		return ""
	}
	u, _ := store.GetUserByID(userID)
	if u == nil {
		return ""
	}
	return u.Username
}

func (a *App) handleMergeMR(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	// merging writes to the repo — require maintainer role (>= 40)
	p := a.principal(r)
	if a.Store.EffectiveRole(p.UserID, repo.ID, repo.OwnerID, p.IsSuper, repo.Visibility) < 40 {
		writeErr(w, http.StatusForbidden, "Maintainer access required")
		return
	}
	num := int(mustPathInt(r, "number"))
	mr, err := a.Store.GetMR(repo.ID, num)
	if err != nil || mr == nil {
		writeErr(w, http.StatusNotFound, "Merge request not found")
		return
	}
	if mr.State != "open" {
		writeErr(w, http.StatusConflict, "Merge request is not open")
		return
	}
	var body struct {
		Method string `json:"method"`
	}
	_ = jsonDecode(r, &body) // method is optional
	if body.Method != "fast-forward" {
		body.Method = "merge-commit"
	}
	dir := a.repoDir(repo)
	sha, err := a.Git.MergeMR(dir, mr.SourceBranch, mr.TargetBranch, body.Method)
	if err != nil {
		writeErr(w, http.StatusConflict, "Merge failed: "+err.Error())
		return
	}
	if err := a.Store.UpdateMR(repo.ID, num, map[string]any{
		"state": "merged", "merge_commit_sha": sha, "updated_at": storage.Now(),
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "merged", "merge_commit_sha": sha})
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
	// Never expose the signing secret to repo readers.
	out := make([]map[string]any, 0, len(hooks))
	for _, h := range hooks {
		out = append(out, map[string]any{
			"id": h.ID, "url": h.URL, "events": h.Events,
			"is_active": h.IsActive, "created_at": h.CreatedAt,
			"has_secret": h.Secret != "",
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	// Managing webhooks is a maintainer-level operation: hooks receive repo
	// events and (once dispatch lands) make the server send HTTP requests,
	// so a plain reader/guest must not create them.
	repo := a.requireRepoWrite(w, r, 40)
	if repo == nil {
		return
	}
	var body struct {
		URL      string   `json:"url"`
		Secret   string   `json:"secret"`
		Events   []string `json:"events"`
		IsActive *bool    `json:"is_active"`
	}
	if err := jsonDecode(r, &body); err != nil || body.URL == "" {
		writeErr(w, http.StatusBadRequest, "url is required")
		return
	}
	// Only http/https webhook targets are permitted (blocks file:// and other
	// schemes that could read local resources when webhooks are dispatched).
	u, err := url.Parse(body.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeErr(w, http.StatusBadRequest, "url must be a valid http(s) URL")
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
	// See handleCreateWebhook: hooks are maintainer-level configuration.
	repo := a.requireRepoWrite(w, r, 40)
	if repo == nil {
		return
	}
	id := mustPathInt(r, "hookID")
	if err := a.Store.DeleteWebhook(repo.ID, id); err != nil {
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
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	p := a.principal(r)
	// Search is pushed down to SQL (LOWER(name)/LOWER(path) LIKE), so it stays
	// fast with thousands of repos instead of scanning everything in memory.
	repos, err := a.Store.SearchRepos(p.UserID, p.IsSuper, q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	out := make([]map[string]any, 0, len(repos))
	for _, rp := range repos {
		out = append(out, repoToMap(rp))
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": out})
}
