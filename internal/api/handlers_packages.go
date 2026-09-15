package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

// --- Packages: generic file registry (npm/docker/maven/pypi metadata shape,
// content-addressed blobs on disk) ---
//
// Reads need repo read access; publishes/deletes need writer role (>=30).
// Blobs live under <BaseDir>/packages/<pkgID>/<fileID>.bin — keyed by
// database IDs, never by client-supplied names, so filename traversal is
// structurally impossible. Uploads stream to disk with a 500MB cap and a
// SHA-256 integrity record verified on download metadata.

// maxPackageBytes caps a single uploaded artifact.
const maxPackageBytes = 500 << 20

func (a *App) packageBlobPath(pkgID, fileID int64) string {
	return filepath.Join(a.Cfg.BaseDir, "packages", strconv.FormatInt(pkgID, 10), strconv.FormatInt(fileID, 10)+".bin")
}

func (a *App) handleListPackages(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	pkgs, err := a.Store.ListPackages(repo.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if pkgs == nil {
		pkgs = []storage.Package{}
	}
	writeJSON(w, http.StatusOK, pkgs)
}

func (a *App) handlePublishPackage(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	p := a.principal(r)
	if a.Store.EffectiveRole(p.UserID, repo.ID, repo.OwnerID, p.IsSuper, repo.Visibility) < 30 && !p.IsSuper {
		writeErr(w, http.StatusForbidden, "Writer access required")
		return
	}
	// Metadata travels in query params (documented contract); the raw body
	// is the artifact bytes. No multipart parsing (attack surface).
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	ptype := strings.TrimSpace(r.URL.Query().Get("type"))
	version := strings.TrimSpace(r.URL.Query().Get("version"))
	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	if ptype == "" {
		ptype = "generic"
	}
	pkgID, err := a.Store.UpsertPackage(repo.ID, name, ptype)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if filename == "" {
		writeJSON(w, http.StatusCreated, map[string]any{"package_id": pkgID})
		return
	}
	// Stream the raw body straight to disk (no RAM buffering).
	if r.ContentLength > maxPackageBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "artifact exceeds 500MB limit")
		return
	}
	if err := os.MkdirAll(filepath.Dir(a.packageBlobPath(pkgID, 0)), 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(a.packageBlobPath(pkgID, 0)), "upload-*")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	tmpName := tmp.Name()
	h := sha256.New()
	n, err := io.CopyN(io.MultiWriter(tmp, h), io.LimitReader(r.Body, maxPackageBytes+1), maxPackageBytes+1)
	_ = tmp.Close()
	if err != nil && err != io.EOF {
		_ = os.Remove(tmpName)
		writeErr(w, http.StatusBadRequest, "failed to read upload")
		return
	}
	if n > maxPackageBytes {
		_ = os.Remove(tmpName)
		writeErr(w, http.StatusRequestEntityTooLarge, "artifact exceeds 500MB limit")
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fileID, err := a.Store.AddPackageFile(pkgID, version, filename, n, sha)
	if err != nil {
		_ = os.Remove(tmpName)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.Rename(tmpName, a.packageBlobPath(pkgID, fileID)); err != nil {
		_ = os.Remove(tmpName)
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"package_id": pkgID, "file_id": fileID, "sha256": sha, "size": n,
	})
}

func (a *App) handleListPackageFiles(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	pkg := a.lookupPackage(w, r, repo)
	if pkg == nil {
		return
	}
	files, err := a.Store.ListPackageFiles(pkg.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if files == nil {
		files = []storage.PackageFile{}
	}
	writeJSON(w, http.StatusOK, files)
}

func (a *App) handleDownloadPackageFile(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	pkg := a.lookupPackage(w, r, repo)
	if pkg == nil {
		return
	}
	version := r.URL.Query().Get("version")
	filename := r.URL.Query().Get("filename")
	f, err := a.Store.GetPackageFile(pkg.ID, version, filename)
	if err != nil || f == nil {
		writeErr(w, http.StatusNotFound, "Not found")
		return
	}
	fh, err := os.Open(a.packageBlobPath(pkg.ID, f.ID))
	if err != nil {
		writeErr(w, http.StatusNotFound, "Not found")
		return
	}
	defer fh.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", f.Filename))
	w.Header().Set("X-Content-SHA256", f.SHA256)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, f.Filename, time.Time{}, fh)
}

func (a *App) handleDeletePackageFile(w http.ResponseWriter, r *http.Request) {
	repo := a.requireRepoAccess(w, r)
	if repo == nil {
		return
	}
	p := a.principal(r)
	if a.Store.EffectiveRole(p.UserID, repo.ID, repo.OwnerID, p.IsSuper, repo.Visibility) < 30 && !p.IsSuper {
		writeErr(w, http.StatusForbidden, "Writer access required")
		return
	}
	pkg := a.lookupPackage(w, r, repo)
	if pkg == nil {
		return
	}
	version := r.URL.Query().Get("version")
	filename := r.URL.Query().Get("filename")
	f, err := a.Store.GetPackageFile(pkg.ID, version, filename)
	if err != nil || f == nil {
		writeErr(w, http.StatusNotFound, "Not found")
		return
	}
	if err := a.Store.DeletePackageFile(pkg.ID, version, filename); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	_ = os.Remove(a.packageBlobPath(pkg.ID, f.ID))
	writeJSON(w, http.StatusOK, map[string]any{"detail": "deleted"})
}

func (a *App) lookupPackage(w http.ResponseWriter, r *http.Request, repo *storage.Repository) *storage.Package {
	name := strings.TrimSpace(r.URL.Query().Get("package"))
	ptype := strings.TrimSpace(r.URL.Query().Get("type"))
	if ptype == "" {
		ptype = "generic"
	}
	if name == "" {
		writeErr(w, http.StatusBadRequest, "package query is required")
		return nil
	}
	pkgs, err := a.Store.ListPackages(repo.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return nil
	}
	for i := range pkgs {
		if pkgs[i].Name == name && pkgs[i].Type == ptype {
			return &pkgs[i]
		}
	}
	writeErr(w, http.StatusNotFound, "Not found")
	return nil
}
