package api

import (
	"net/http"
	"strings"

	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

func (a *App) handleListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := a.Store.ListGroups(a.principal(r).UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if groups == nil {
		groups = []storage.Group{}
	}
	writeJSON(w, http.StatusOK, groups)
}

func (a *App) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	var body struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Description string `json:"description"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	path := strings.TrimSpace(body.Path)
	if path == "" {
		path = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	}
	if !validRepoName(path) {
		writeErr(w, http.StatusBadRequest, "Invalid group path")
		return
	}
	id, err := a.Store.CreateGroup(name, path, body.Description, p.UserID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "Group path already in use")
		return
	}
	g, _ := a.Store.GetGroup(id)
	a.Store.AddAuditEvent("group.create", p.UserID, p.Username, "group", path, "Group "+path+" created")
	writeJSON(w, http.StatusCreated, g)
}

func (a *App) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	id := mustPathInt(r, "id")
	g, err := a.Store.GetGroup(id)
	if err != nil || g == nil {
		writeErr(w, http.StatusNotFound, "Group not found")
		return
	}
	g.MemberCount = a.Store.CountGroupMembers(id)
	writeJSON(w, http.StatusOK, g)
}

func (a *App) handleListGroupProjects(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "id")
	g, err := a.Store.GetGroup(id)
	if err != nil || g == nil {
		writeErr(w, http.StatusNotFound, "Group not found")
		return
	}
	repos, err := a.Store.ListGroupProjects(id, p.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	out := make([]map[string]any, 0, len(repos))
	for _, rp := range repos {
		out = append(out, repoToMap(rp))
	}
	writeJSON(w, http.StatusOK, out)
}
