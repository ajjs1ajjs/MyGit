package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
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

// TestSecondUserIsNotSuperuser guards the authorization model: only the first
// registered user may be promoted to superuser.
func TestSecondUserIsNotSuperuser(t *testing.T) {
	_, base, _ := newTestApp(t)

	// first user -> superuser
	resp, b := authReq("GET", base+"/api/v1/users/me/", registerAndLogin(t, base), nil)
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"is_superuser":true`) {
		t.Fatalf("first user should be superuser: %d %s", resp.StatusCode, b)
	}

	// second user -> regular
	resp2, err := http.Post(base+"/api/v1/auth/register/", "application/json",
		strings.NewReader(`{"username":"bob","email":"bob@example.com","password":"password123"}`))
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 201 {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("register bob = %d: %s", resp2.StatusCode, b)
	}
	var login struct {
		Access string `json:"access"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&login)
	resp3, b := authReq("GET", base+"/api/v1/users/me/", login.Access, nil)
	if resp3.StatusCode != 200 {
		t.Fatalf("me bob = %d", resp3.StatusCode)
	}
	if strings.Contains(string(b), `"is_superuser":true`) {
		t.Fatalf("second user must not be superuser: %s", b)
	}
}

// TestCannotDeleteOthersToken guards against the IDOR where any authenticated
// user could delete another user's PAT by guessing its id.
func TestCannotDeleteOthersToken(t *testing.T) {
	_, base, _ := newTestApp(t)
	aliceToken := registerAndLogin(t, base)

	// alice creates a token -> get its id
	resp, b := authReq("POST", base+"/api/v1/users/alice/tokens/", aliceToken, map[string]any{"name": "ci", "scopes": []string{"read"}})
	if resp.StatusCode != 201 {
		t.Fatalf("create token = %d: %s", resp.StatusCode, b)
	}
	var created map[string]any
	_ = json.Unmarshal(b, &created)
	tokenID := created["id"]
	rawPAT, _ := created["token"].(string)
	if rawPAT == "" {
		t.Fatalf("create token did not return raw token: %s", b)
	}

	// the raw PAT must authenticate (regression: NULL last_used_at/expires_at
	// used to break token lookup on a fresh token)
	resp, b = authReq("GET", base+"/api/v1/users/me/", rawPAT, nil)
	if resp.StatusCode != 200 || !strings.Contains(string(b), "alice") {
		t.Fatalf("PAT auth failed: %d %s", resp.StatusCode, b)
	}

	// bob registers and tries to delete alice's token
	http.Post(base+"/api/v1/auth/register/", "application/json",
		strings.NewReader(`{"username":"bob","email":"bob@example.com","password":"password123"}`))
	var login struct {
		Access string `json:"access"`
	}
	respLogin, err := http.Post(base+"/api/v1/auth/login/", "application/json",
		strings.NewReader(`{"username":"bob","password":"password123"}`))
	if err != nil {
		t.Fatalf("bob login: %v", err)
	}
	defer respLogin.Body.Close()
	_ = json.NewDecoder(respLogin.Body).Decode(&login)

	path := fmt.Sprintf("%s/api/v1/users/alice/tokens/%v/", base, tokenID)
	resp, b = authReq("DELETE", path, login.Access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("bob delete = %d: %s", resp.StatusCode, b)
	}

	// alice's token must still be listed
	resp, b = authReq("GET", base+"/api/v1/users/alice/tokens/", aliceToken, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list tokens = %d", resp.StatusCode)
	}
	if !strings.Contains(string(b), "ci") {
		t.Fatalf("alice's token was deleted by bob: %s", b)
	}
}

// TestGitRepoPathTraversal guards gitRepoFromPath against path traversal via
// owner/repo URL parameters (../, %2F, encoded dots). The invariant is that a
// traversal attempt must never yield a git smart-HTTP advertisement (a 200
// with application/x-git content / "# service=git-" body). It may hit the SPA
// fallback (harmless static HTML), but never a git operation.
func TestGitRepoPathTraversal(t *testing.T) {
	_, base, _ := newTestApp(t)
	for _, attempt := range []string{
		"/../etc/info/refs?service=git-upload-pack",
		"/..%2F..%2Fetc/info/refs?service=git-upload-pack",
		"/%2e%2e/%2e%2e/etc/info/refs?service=git-upload-pack",
		"/alice/%2e%2e/info/refs?service=git-upload-pack",
		"/../etc/git-upload-pack",
	} {
		resp, err := http.Get(base + attempt)
		if err != nil {
			t.Fatalf("GET %s: %v", attempt, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if strings.Contains(resp.Header.Get("Content-Type"), "application/x-git") {
			t.Fatalf("traversal %s leaked a git advertisement (status %d): %s", attempt, resp.StatusCode, body)
		}
		if strings.Contains(string(body), "# service=git-") {
			t.Fatalf("traversal %s leaked a git advertisement body: %s", attempt, body)
		}
	}
}

// TestIssueNumbersAreSequential ensures the atomic MAX+1 insert keeps
// per-repository issue numbers unique and monotonic across creations.
func TestIssueNumbersAreSequential(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)
	authReq("POST", base+"/api/v1/projects/", token, map[string]any{"name": "seq"})

	for i := 1; i <= 5; i++ {
		resp, b := authReq("POST", base+"/api/v1/projects/1/issues/", token, map[string]any{"title": "issue"})
		if resp.StatusCode != 201 {
			t.Fatalf("create issue %d = %d: %s", i, resp.StatusCode, b)
		}
		var created map[string]any
		_ = json.Unmarshal(b, &created)
		if created["number"] != float64(i) {
			t.Fatalf("issue %d number = %v, want %d", i, created["number"], i)
		}
	}
}

// TestTokenRevocationOnPasswordChange guards JWT revocation: after a password
// change the user's token_version is bumped, so previously issued access tokens
// must be rejected.
func TestTokenRevocationOnPasswordChange(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)

	// sanity: old token works
	resp, _ := authReq("GET", base+"/api/v1/users/me/", token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("me before change = %d", resp.StatusCode)
	}

	// change password (bumps token_version)
	resp, b := authReq("POST", base+"/api/v1/users/change_password/", token, map[string]any{
		"current_password": "password123", "new_password": "newpass45678",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("change password = %d: %s", resp.StatusCode, b)
	}

	// old access token must now be rejected
	resp, b = authReq("GET", base+"/api/v1/users/me/", token, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("old token after password change = %d, want 401 (%s)", resp.StatusCode, b)
	}

	// login with new password issues a working token
	var login struct {
		Access string `json:"access"`
	}
	respLogin, err := http.Post(base+"/api/v1/auth/login/", "application/json",
		strings.NewReader(`{"username":"alice","password":"newpass45678"}`))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer respLogin.Body.Close()
	_ = json.NewDecoder(respLogin.Body).Decode(&login)
	resp, b = authReq("GET", base+"/api/v1/users/me/", login.Access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("new token after change = %d: %s", resp.StatusCode, b)
	}
}

// gitCmd runs a git command and fails the test on error.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

// TestMergeMR creates a real feature branch, opens a merge request against main
// and verifies that merging produces a merge commit on the target branch (this
// used to be a no-op that only flipped the DB state).
func TestMergeMR(t *testing.T) {
	_, base, repoRoot := newTestApp(t)
	token := registerAndLogin(t, base)
	authReq("POST", base+"/api/v1/projects/", token, map[string]any{"name": "merge-me"})

	repoDir := filepath.Join(repoRoot, "alice", "merge-me.git")

	// clone the bare repo, make commits on main and a feature branch
	work := t.TempDir()
	gitCmd(t, work, "clone", "-q", repoDir, filepath.Join(work, "wc"))
	wc := filepath.Join(work, "wc")
	gitCmd(t, wc, "config", "user.email", "alice@example.com")
	gitCmd(t, wc, "config", "user.name", "alice")
	gitCmd(t, wc, "checkout", "-q", "-b", "main")
	os.WriteFile(filepath.Join(wc, "a.txt"), []byte("hello"), 0o644)
	gitCmd(t, wc, "add", ".")
	gitCmd(t, wc, "commit", "-q", "-m", "initial")
	gitCmd(t, wc, "push", "-q", "-u", "origin", "main")

	// feature branch with a new commit
	gitCmd(t, wc, "checkout", "-q", "-b", "feature")
	os.WriteFile(filepath.Join(wc, "b.txt"), []byte("world"), 0o644)
	gitCmd(t, wc, "add", ".")
	gitCmd(t, wc, "commit", "-q", "-m", "feature work")
	gitCmd(t, wc, "push", "-q", "-u", "origin", "feature")

	// open a merge request feature -> main
	resp, b := authReq("POST", base+"/api/v1/projects/1/merge_requests/", token, map[string]any{
		"source_branch": "feature", "target_branch": "main", "title": "merge feature",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create MR = %d: %s", resp.StatusCode, b)
	}

	// merge it
	resp, b = authReq("POST", base+"/api/v1/projects/1/merge_requests/1/merge/", token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("merge MR = %d: %s", resp.StatusCode, b)
	}

	// target branch must now contain the feature commit (real merge, not a stub)
	// --no-ff creates a merge commit, so check ancestry rather than SHA equality.
	gitCmd(t, repoDir, "merge-base", "--is-ancestor", "feature", "main")
	ancestryLog := gitCmd(t, repoDir, "log", "--oneline", "main")
	if !strings.Contains(ancestryLog, "Merge branch 'feature'") {
		t.Fatalf("expected a merge commit on main, got log:\n%s", ancestryLog)
	}
}

// TestCookieSession verifies the SPA flow: login sets HttpOnly session cookies
// and authenticated requests work via the cookie jar (no Authorization header).
func TestCookieSession(t *testing.T) {
	_, base, _ := newTestApp(t)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	regBody := `{"username":"cookie_user","email":"cookie@example.com","password":"password123"}`
	resp, err := client.Post(base+"/api/v1/auth/register/", "application/json", strings.NewReader(regBody))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("register = %d", resp.StatusCode)
	}
	// cookies must be HttpOnly (not readable by JS)
	for _, c := range resp.Cookies() {
		if !c.HttpOnly {
			t.Fatalf("cookie %q must be HttpOnly", c.Name)
		}
		if c.SameSite != http.SameSiteStrictMode {
			t.Fatalf("cookie %q must be SameSite=Strict", c.Name)
		}
	}

	// /me authenticates via cookies alone
	req, _ := http.NewRequest("GET", base+"/api/v1/users/me/", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(b), "cookie_user") {
		t.Fatalf("cookie /me = %d: %s", resp.StatusCode, b)
	}

	// logout clears the session
	resp, err = client.Post(base+"/api/v1/auth/logout/", "application/json", nil)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	resp.Body.Close()
	resp, err = client.Get(base + "/api/v1/users/me/")
	if err != nil {
		t.Fatalf("me after logout: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("me after logout = %d, want 401", resp.StatusCode)
	}
}

var appRepoRoot string
