package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

func (a *App) withAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := a.principal(r)
		if p == nil || !p.IsSuper {
			writeErr(w, http.StatusForbidden, "Admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- audit log ---

func (a *App) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	events, err := a.Store.ListAuditEvents(r.URL.Query().Get("action"), 100)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if events == nil {
		events = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, events)
}

// --- backup schedules & jobs ---

func (a *App) handleBackupSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := a.Store.ListBackupSchedules()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if schedules == nil {
		schedules = []storage.BackupSchedule{}
	}
	writeJSON(w, http.StatusOK, schedules)
}

func (a *App) handleCreateBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string `json:"name"`
		Frequency string `json:"frequency"`
		TimeOfDay string `json:"time_of_day"`
		Enabled   *bool  `json:"enabled"`
		Encrypt   *bool  `json:"encrypt"`
		Upload    *bool  `json:"upload"`
		KeepLocal int    `json:"keep_local"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	s := &storage.BackupSchedule{
		Name:      strings.TrimSpace(body.Name),
		Frequency: body.Frequency,
		TimeOfDay: body.TimeOfDay,
		Enabled:   1,
		Encrypt:   1,
		Upload:    1,
		KeepLocal: body.KeepLocal,
	}
	if body.Frequency == "" {
		s.Frequency = "daily"
	}
	if body.TimeOfDay == "" {
		s.TimeOfDay = "02:15:00"
	}
	if body.Enabled != nil {
		s.Enabled = b2i(*body.Enabled)
	}
	if body.Encrypt != nil {
		s.Encrypt = b2i(*body.Encrypt)
	}
	if body.Upload != nil {
		s.Upload = b2i(*body.Upload)
	}
	if s.KeepLocal <= 0 {
		s.KeepLocal = 14
	}
	id, err := a.Store.CreateBackupSchedule(s)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": s.Name})
}

func (a *App) handlePatchBackupSchedule(w http.ResponseWriter, r *http.Request) {
	id := mustPathInt(r, "id")
	s, err := a.Store.GetBackupSchedule(id)
	if err != nil || s == nil {
		writeErr(w, http.StatusNotFound, "Schedule not found")
		return
	}
	var body map[string]any
	if err := jsonDecode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	fields := map[string]any{}
	if v, ok := body["enabled"].(bool); ok {
		fields["enabled"] = b2i(v)
	}
	if v, ok := body["name"].(string); ok {
		fields["name"] = v
	}
	if v, ok := body["frequency"].(string); ok {
		fields["frequency"] = v
	}
	if err := a.Store.UpdateBackupSchedule(id, fields); err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"detail": "updated"})
}

func (a *App) handleRunBackupNow(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "id")
	s, err := a.Store.GetBackupSchedule(id)
	if err != nil || s == nil {
		writeErr(w, http.StatusNotFound, "Schedule not found")
		return
	}
	jobID, err := a.Store.CreateBackupJob("scheduled")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	a.Store.AddAuditEvent("backup.run", p.UserID, p.Username, "backup", s.Name, "Backup schedule '"+s.Name+"' triggered")
	go a.runBackup(jobID, s.Name)
	writeJSON(w, http.StatusOK, map[string]any{"job_id": jobID, "detail": "backup started"})
}

func (a *App) handleBackupJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := a.Store.ListBackupJobs()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if jobs == nil {
		jobs = []storage.BackupJob{}
	}
	writeJSON(w, http.StatusOK, jobs)
}

// runBackup archives the data directory (DB + repos + secrets) into
// {base}/backups/ as a tar.gz. Runs in a goroutine and updates the job row.
func (a *App) runBackup(jobID int64, scheduleName string) {
	archivePath, err := a.createBackupArchive()
	if err != nil {
		a.Store.FinishBackupJob(jobID, "failed", "", err.Error())
		return
	}
	a.Store.FinishBackupJob(jobID, "success", archivePath, "")
}

func (a *App) createBackupArchive() (string, error) {
	backupDir := filepath.Join(a.Cfg.BaseDir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}
	name := "mygit-backup-" + time.Now().UTC().Format("20060102-150405") + ".tar.gz"
	archivePath := filepath.Join(backupDir, name)

	f, err := os.Create(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	addFile := func(path string) error {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(a.Cfg.BaseDir, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(tw, src)
		return err
	}

	// DB files
	_ = addFile(a.Cfg.DBPath)
	_ = addFile(a.Cfg.DBPath + "-wal")
	_ = addFile(a.Cfg.DBPath + "-shm")
	_ = addFile(filepath.Join(filepath.Dir(a.Cfg.DBPath), ".mygit_jwt_secret"))

	// Repos (bare repo dirs)
	if repoRoot := a.Cfg.RepoRoot; repoRoot != "" {
		_ = filepath.Walk(repoRoot, func(p string, info os.FileInfo, err error) error {
			if err != nil || !info.Mode().IsDir() {
				return nil
			}
			if p == repoRoot {
				return nil
			}
			return filepath.Walk(p, func(f string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() {
					return nil
				}
				return addFile(f)
			})
		})
	}

	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return archivePath, nil
}

// --- mirror targets ---

func (a *App) handleMirrorTargets(w http.ResponseWriter, r *http.Request) {
	mirrors, err := a.Store.ListMirrorTargets()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if mirrors == nil {
		mirrors = []storage.MirrorTarget{}
	}
	writeJSON(w, http.StatusOK, mirrors)
}

func (a *App) handleSyncMirror(w http.ResponseWriter, r *http.Request) {
	p := a.principal(r)
	id := mustPathInt(r, "id")
	m, err := a.Store.GetMirrorTarget(id)
	if err != nil || m == nil {
		writeErr(w, http.StatusNotFound, "Mirror target not found")
		return
	}
	a.Store.AddAuditEvent("mirror.sync", p.UserID, p.Username, "mirror", m.Name, "Mirror '"+m.Name+"' sync started")
	go a.syncMirror(m)
	writeJSON(w, http.StatusOK, map[string]any{"detail": "sync started"})
}

// syncMirror pushes every local repository's refs (--mirror) to
// <target>/<owner>/<name>.git, creating bare repos there on first run. This
// makes mirror targets work as a local bare-repo store.
func (a *App) syncMirror(m *storage.MirrorTarget) {
	repos, err := a.Store.ListAccessibleRepos(0, true)
	if err != nil {
		a.Store.SetMirrorResult(m.ID, "failed", err.Error())
		return
	}
	targetRoot := m.Target
	if targetRoot == "" {
		a.Store.SetMirrorResult(m.ID, "failed", "empty target")
		return
	}
	_ = os.MkdirAll(targetRoot, 0o755)
	failed := 0
	for _, rp := range repos {
		owner, name := repoPathParts(rp.Path)
		if owner == "" || name == "" {
			continue
		}
		src := a.Git.RepoPath(owner, name)
		dst := filepath.Join(targetRoot, owner, name+".git")
		if _, err := os.Stat(filepath.Join(dst, "HEAD")); err != nil {
			_ = os.MkdirAll(filepath.Dir(dst), 0o755)
			_ = a.runGitInit(dst, rp.DefaultBranch)
		}
		if err := a.Git.PushMirror(src, dst); err != nil {
			failed++
			continue
		}
	}
	if failed > 0 {
		a.Store.SetMirrorResult(m.ID, "failed", fmt.Sprintf("%d repo(s) failed to sync", failed))
		return
	}
	a.Store.SetMirrorResult(m.ID, "success", "")
}

func (a *App) runGitInit(dir, branch string) error {
	if branch == "" {
		branch = "main"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.Cfg.GitBinary, "init", "--bare", "--initial-branch="+branch, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init: %v: %s", err, out)
	}
	return nil
}

// --- import jobs (admin read-only listing) ---

func (a *App) handleImportJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := a.Store.ListImportJobs()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Database error")
		return
	}
	if jobs == nil {
		jobs = []storage.ImportJob{}
	}
	writeJSON(w, http.StatusOK, jobs)
}
