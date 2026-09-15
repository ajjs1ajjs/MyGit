package api

import (
	"net/http"
	"path"
	"strings"
)

// --- Pages: static site hosting straight from git objects ---
//
// No checkout, no filesystem reads outside the bare repo: every file is
// served via `git cat-file` inside the object database, so path traversal
// is structurally impossible (there is no Join with user input). Only the
// repository default branch is served; directory requests fall back to
// index.html (SPA-friendly). Only public/internal repos are servable —
// private repos 404 for anonymous callers (readers with access pass the
// normal repo gate).

func (a *App) handlePages(w http.ResponseWriter, r *http.Request) {
	owner := urlParam(r, "owner")
	name := strings.TrimSuffix(urlParam(r, "repo"), ".git")
	if !validRepoName(owner) || !validRepoName(name) {
		writeErr(w, http.StatusNotFound, "Not found")
		return
	}
	repo, err := a.Store.GetRepoByPath(owner + "/" + name)
	if err != nil || repo == nil {
		writeErr(w, http.StatusNotFound, "Not found")
		return
	}
	// Visibility gate: anonymous callers only get public repos; authed
	// callers need read access like everywhere else.
	if p, aerr := a.authenticate(r); aerr != nil {
		if repo.Visibility != "public" {
			writeErr(w, http.StatusNotFound, "Not found")
			return
		}
	} else if role := a.Store.EffectiveRole(p.UserID, repo.ID, repo.OwnerID, p.IsSuper, repo.Visibility); role < 10 {
		writeErr(w, http.StatusNotFound, "Not found")
		return
	}
	dir := a.repoDir(repo)
	branch := repo.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	if !safeRefArg(branch) {
		writeErr(w, http.StatusNotFound, "Not found")
		return
	}
	// Clean the URL path inside the repo (no filesystem involved, but keep
	// the `..` segments from confusing tree lookups).
	rel := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/-/pages/"+owner+"/"+name+"/"))
	rel = strings.TrimPrefix(rel, "/")
	try := []string{rel}
	if rel == "" || strings.HasSuffix(r.URL.Path, "/") {
		try = []string{"index.html"}
	} else {
		try = append(try, strings.TrimSuffix(rel, "/")+"/index.html")
	}
	for _, p := range try {
		data, err := a.Git.Blob(dir, branch, p)
		if err != nil {
			continue
		}
		w.Header().Set("Content-Type", contentTypeFor(p))
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}
	writeErr(w, http.StatusNotFound, "Not found")
}

func contentTypeFor(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".txt", ".md":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
