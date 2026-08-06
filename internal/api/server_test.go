package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ajjs1ajjs/MyGit/internal/auth"
	"github.com/ajjs1ajjs/MyGit/internal/config"
	"github.com/ajjs1ajjs/MyGit/internal/git"
	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

func newTestApp(t *testing.T) (*App, string, string) {
	t.Helper()
	dir := t.TempDir()
	db, abs, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	store := storage.NewStore(db, abs)
	t.Cleanup(func() { _ = store.DB.Close() })

	cfg := config.Default()
	cfg.RepoRoot = filepath.Join(dir, "repos")
	cfg.JWTSecret = "test-secret-1234567890abcdefghijklmnopqrstuvwxyz"
	cfg.InternalToken = "internal-test-token"
	_ = os.MkdirAll(cfg.RepoRoot, 0o755)

	authn := auth.New(cfg.JWTSecret, 15, 30)
	gitBackend := git.New("git", cfg.RepoRoot)
	app := &App{Cfg: cfg, Store: store, Auth: authn, Git: gitBackend, Start: time.Now()}
	srv := httptest.NewServer(app.Handler())
	t.Cleanup(srv.Close)
	return app, srv.URL, cfg.RepoRoot
}

func registerAndLogin(t *testing.T, base string) string {
	t.Helper()
	body := `{"username":"alice","email":"alice@example.com","password":"password123"}`
	resp, err := http.Post(base+"/api/v1/auth/register/", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("register = %d: %s", resp.StatusCode, b)
	}
	var res struct {
		Access string `json:"access"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if res.Access == "" {
		t.Fatalf("no access token")
	}
	return res.Access
}

func authReq(method, url, token string, body any) (*http.Response, []byte) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func TestRegisterLoginAndMe(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)
	resp, b := authReq("GET", base+"/api/v1/users/me/", token, nil)
	if resp.StatusCode != 200 || !strings.Contains(string(b), "alice") {
		t.Fatalf("me = %d: %s", resp.StatusCode, b)
	}
	// unauthenticated rejected
	resp, _ = http.Get(base + "/api/v1/users/me/")
	if resp.StatusCode != 401 {
		t.Fatalf("unauth me = %d", resp.StatusCode)
	}
}

func TestCreateAndListProject(t *testing.T) {
	_, base, repoRoot := newTestApp(t)
	token := registerAndLogin(t, base)

	// create project -> bare repo on disk
	resp, b := authReq("POST", base+"/api/v1/projects/", token, map[string]any{
		"name": "my-repo", "visibility": "private",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create = %d: %s", resp.StatusCode, b)
	}
	var created map[string]any
	_ = json.Unmarshal(b, &created)
	if created["path"] != "alice/my-repo" {
		t.Fatalf("path = %v", created["path"])
	}

	// repo dir exists
	if _, err := os.Stat(filepath.Join(repoRoot, "alice", "my-repo.git", "HEAD")); err != nil {
		t.Fatalf("bare repo not created: %v", err)
	}

	// list projects
	resp, b = authReq("GET", base+"/api/v1/projects/", token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list = %d", resp.StatusCode)
	}
	if !strings.Contains(string(b), "my-repo") {
		t.Fatalf("list missing repo: %s", b)
	}

	// by-path lookup
	resp, b = authReq("GET", base+"/api/v1/projects/by-path/alice/my-repo/", token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("by-path = %d: %s", resp.StatusCode, b)
	}
}

func TestGitSmartHTTPInfoRefs(t *testing.T) {
	// git smart HTTP relies on the git binary finding its git-upload-pack
	// sub-command. On some Windows test harnesses git can't resolve it; on
	// Linux (production) it always works. Skip when it can't resolve.
	if err := exec.Command("git", "upload-pack", "--help").Run(); err != nil {
		t.Skipf("git upload-pack unavailable in this environment: %v", err)
	}
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)
	authReq("POST", base+"/api/v1/projects/", token, map[string]any{"name": "git-repo", "visibility": "public"})

	// anonymous info/refs on a public repo (upload-pack)
	resp, err := http.Get(base + "/alice/git-repo.git/info/refs?service=git-upload-pack")
	if err != nil {
		t.Fatalf("info/refs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("info/refs = %d: %s", resp.StatusCode, b)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "git-upload-pack") {
		t.Fatalf("content-type = %q", ct)
	}

	// receive-pack requires auth
	resp, err = http.Get(base + "/alice/git-repo.git/info/refs?service=git-receive-pack")
	if err != nil {
		t.Fatalf("receive-pack: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("anon receive-pack = %d, want 401", resp.StatusCode)
	}
}

func TestIssuesLifecycle(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)
	authReq("POST", base+"/api/v1/projects/", token, map[string]any{"name": "bug-tracker"})

	// create issue
	resp, b := authReq("POST", base+"/api/v1/projects/1/issues/", token, map[string]any{
		"title": "First bug", "description": "details",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create issue = %d: %s", resp.StatusCode, b)
	}
	var created map[string]any
	_ = json.Unmarshal(b, &created)
	if created["number"] != float64(1) {
		t.Fatalf("issue number = %v", created["number"])
	}

	// list issues
	resp, b = authReq("GET", base+"/api/v1/projects/1/issues/", token, nil)
	if resp.StatusCode != 200 || !strings.Contains(string(b), "First bug") {
		t.Fatalf("list issues = %d: %s", resp.StatusCode, b)
	}

	// comment
	resp, b = authReq("POST", base+"/api/v1/projects/1/issues/1/comments/", token, map[string]any{"body": "me too"})
	if resp.StatusCode != 201 {
		t.Fatalf("comment = %d: %s", resp.StatusCode, b)
	}
}

func TestEmptyListsAreArrays(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)
	resp, b := authReq("GET", base+"/api/v1/projects/", token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list = %d", resp.StatusCode)
	}
	if strings.Contains(string(b), `"results":null`) {
		t.Fatalf("empty projects returned null: %s", b)
	}
}

var appRepoRoot string
