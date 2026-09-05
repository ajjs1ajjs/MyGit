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
	"sort"
	"strconv"
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
	go a.runBackup(jobID, s)
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
// {base}/backups/, honoring the schedule's encrypt/upload/keep_local settings.
// Runs in a goroutine and updates the job row.
func (a *App) runBackup(jobID int64, s *storage.BackupSchedule) {
	archivePath, err := a.createBackupArchive()
	if err != nil {
		a.Store.FinishBackupJob(jobID, "failed", "", err.Error())
		return
	}
	resultPath := archivePath
	notes := ""

	// Encryption (AES-256-GCM, key from MYGIT_BACKUP_KEY or derived).
	if s.Encrypt == 1 {
		encPath, err := encryptFile(archivePath, a.Cfg.JWTSecret, a.Cfg.BackupKey)
		if err != nil {
			a.Store.FinishBackupJob(jobID, "failed", archivePath, "encrypt: "+err.Error())
			return
		}
		resultPath = encPath
		notes = "encrypted"
	}

	// Upload (HTTP PUT to MYGIT_BACKUP_UPLOAD_URL).
	if s.Upload == 1 {
		if a.Cfg.BackupUploadURL == "" {
			notes += "; upload skipped (MYGIT_BACKUP_UPLOAD_URL not set)"
		} else if err := a.uploadBackup(resultPath); err != nil {
			notes += "; upload failed: " + err.Error()
		} else {
			notes += "; uploaded"
		}
	}

	// Keep only the newest keep_local archives.
	if s.KeepLocal > 0 {
		a.pruneBackups(s.KeepLocal)
	}

	a.Store.FinishBackupJob(jobID, "success", resultPath, notes)
}

// StartBackupScheduler launches the background loop that runs enabled backup
// schedules when their time_of_day/frequency comes due. Called once at startup.
func (a *App) StartBackupScheduler() {
	go a.backupSchedulerLoop()
}

func (a *App) backupSchedulerLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.runDueBackups(time.Now().UTC())
	}
}

// runDueBackups checks every enabled schedule and launches backups that are due.
func (a *App) runDueBackups(now time.Time) {
	schedules, err := a.Store.ListBackupSchedules()
	if err != nil {
		return
	}
	for _, s := range schedules {
		if s.Enabled != 1 {
			continue
		}
		next, ok := nextBackupTime(&s, now)
		if !ok || now.Before(next) || now.After(next.Add(backupGraceWindow)) {
			continue
		}
		// Reserve the run before spawning the goroutine so the next tick can't
		// double-fire.
		_ = a.Store.UpdateBackupSchedule(s.ID, map[string]any{"last_run_at": storage.Now()})
		jobID, err := a.Store.CreateBackupJob(s.Name)
		if err != nil {
			continue
		}
		a.Store.AddAuditEvent("backup.scheduled", 0, "system", "backup", s.Name, "Scheduled backup '"+s.Name+"' started")
		go a.runBackup(jobID, &s)
	}
}

// backupGraceWindow is how long after a slot the scheduler still considers it
// runnable before skipping to the next future slot.
const backupGraceWindow = 10 * time.Minute

// nextBackupTime returns the next occurrence of the schedule's time after its
// last run (or after creation when it has never run). A never-run schedule
// whose slot is already older than the grace window waits for the next future
// slot instead of catching up.
func nextBackupTime(s *storage.BackupSchedule, now time.Time) (time.Time, bool) {
	h, m, sec := parseTimeOfDay(s.TimeOfDay)
	base := now
	if s.LastRunAt != "" {
		if t, err := parseStorageTime(s.LastRunAt); err == nil {
			base = t
		}
	} else if s.CreatedAt != "" {
		if t, err := parseStorageTime(s.CreatedAt); err == nil {
			base = t
		}
	}
	next := nextSlotAfter(base, s.Frequency, h, m, sec)
	if s.LastRunAt == "" && now.Sub(next) > backupGraceWindow {
		next = nextSlotAfter(now, s.Frequency, h, m, sec)
	}
	return next, true
}

// nextSlotAfter returns the first slot of the given frequency strictly after
// base.
func nextSlotAfter(base time.Time, frequency string, h, m, sec int) time.Time {
	switch frequency {
	case "hourly":
		next := time.Date(base.Year(), base.Month(), base.Day(), base.Hour(), m, sec, 0, time.UTC)
		if !next.After(base) {
			next = next.Add(time.Hour)
		}
		return next
	case "weekly":
		next := time.Date(base.Year(), base.Month(), base.Day(), h, m, sec, 0, time.UTC)
		if !next.After(base) {
			next = next.AddDate(0, 0, 7)
		}
		return next
	default: // daily
		next := time.Date(base.Year(), base.Month(), base.Day(), h, m, sec, 0, time.UTC)
		if !next.After(base) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	}
}

// parseTimeOfDay parses "HH:MM" or "HH:MM:SS".
func parseTimeOfDay(tod string) (h, m, sec int) {
	parts := strings.Split(tod, ":")
	if len(parts) > 0 {
		h, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	}
	if len(parts) > 1 {
		m, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	}
	if len(parts) > 2 {
		sec, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
	}
	return
}

// parseStorageTime parses the storage UTC timestamp (or RFC3339).
func parseStorageTime(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02T15:04:05.000000+00:00", s)
	if err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// uploadBackup PUTs a file to MYGIT_BACKUP_UPLOAD_URL. The URL may contain a
// {filename} placeholder, end with "/" (filename is appended), or be used as
// the exact destination (e.g. a presigned S3 URL).
func (a *App) uploadBackup(path string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	base := strings.TrimRight(a.Cfg.BackupUploadURL, "/")
	filename := filepath.Base(path)
	if strings.Contains(base, "{filename}") {
		base = strings.ReplaceAll(base, "{filename}", filename)
	} else if !strings.HasSuffix(base, "/") && !strings.Contains(base, "?") {
		base = base + "/" + filename
	}
	req, err := http.NewRequest(http.MethodPut, base, f)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("upload returned %d", resp.StatusCode)
	}
	return nil
}

// pruneBackups removes the oldest archives in the backups dir, keeping the
// newest keepLocal files.
func (a *App) pruneBackups(keepLocal int) {
	backupDir := filepath.Join(a.Cfg.BaseDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return
	}
	type f struct {
		name string
		mod  time.Time
	}
	var files []f
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasPrefix(e.Name(), "mygit-backup-") {
			continue
		}
		if info, err := e.Info(); err == nil {
			files = append(files, f{name: e.Name(), mod: info.ModTime()})
		}
	}
	if len(files) <= keepLocal {
		return
	}
	// newest first
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	for _, old := range files[keepLocal:] {
		_ = os.Remove(filepath.Join(backupDir, old.name))
	}
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

	addFileNamed := func(path, name string) error {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = name
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
	addFile := func(path string) error {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(a.Cfg.BaseDir, path)
		if err != nil {
			return err
		}
		return addFileNamed(path, filepath.ToSlash(rel))
	}

	// DB snapshot: VACUUM INTO writes a consistent copy of the live database
	// (raw db+wal+shm copies used to be torn when writes happened during the
	// backup). Falls back to the raw files if the snapshot fails.
	snapshot := filepath.Join(backupDir, fmt.Sprintf("db-snapshot-%d.db", time.Now().UnixNano()))
	if _, err := a.Store.DB.Exec("VACUUM INTO ?", snapshot); err == nil {
		_ = addFile(snapshot)
		_ = os.Remove(snapshot)
	} else {
		_ = addFile(a.Cfg.DBPath)
		_ = addFile(a.Cfg.DBPath + "-wal")
		_ = addFile(a.Cfg.DBPath + "-shm")
	}
	_ = addFile(filepath.Join(filepath.Dir(a.Cfg.DBPath), ".mygit_jwt_secret"))

	// Repos (bare repo dirs)
	if repoRoot := a.Cfg.RepoRoot; repoRoot != "" {
		_ = filepath.Walk(repoRoot, func(p string, info os.FileInfo, err error) error {
			if err != nil || !info.Mode().IsRegular() {
				return nil
			}
			return addFile(p)
		})
	}

	// Repos stored at custom locations outside RepoRoot must be backed up
	// too — they used to be a silent data-loss blind spot. They are archived
	// under a custom-storage/ prefix so archive names stay unambiguous.
	if custom, err := a.Store.ListCustomStorageRepos(); err == nil {
		for _, rp := range custom {
			dir, ok := a.allowedCustomDir(rp.StoragePath)
			if !ok || dir == "" {
				continue
			}
			base := filepath.Base(dir)
			_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
				if err != nil || !info.Mode().IsRegular() {
					return nil
				}
				rel, relErr := filepath.Rel(dir, p)
				if relErr != nil {
					return nil
				}
				return addFileNamed(p, "custom-storage/"+base+"/"+filepath.ToSlash(rel))
			})
		}
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
