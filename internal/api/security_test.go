package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRefOptionInjectionRejected: a ref/sha argument starting with "-" used
// to be passed to git verbatim, letting any reader trigger
// `git log --output=<path>` (arbitrary file write). It must be rejected
// before it ever reaches a git command line.
func TestRefOptionInjectionRejected(t *testing.T) {
	_, base, repoRoot := newTestApp(t)
	token := registerAndLogin(t, base)
	authReq("POST", base+"/api/v1/projects/", token, map[string]any{"name": "opts", "visibility": "public"})
	pushFile(t, filepath.Join(repoRoot, "alice", "opts.git"), "a.txt", "x")

	target := filepath.ToSlash(filepath.Join(t.TempDir(), "pwned.txt"))
	resp, b := authReq("GET", base+"/api/v1/projects/1/commits/?ref="+url.QueryEscape("--output="+target), token, nil)
	if resp.StatusCode != 400 {
		t.Fatalf("commits with option ref = %d (%s), want 400", resp.StatusCode, b)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatalf("git wrote the --output file: arbitrary file write!")
	}

	// commit detail: a relative "--output=..." must also be rejected
	resp, _ = authReq("GET", base+"/api/v1/projects/1/commits/"+url.PathEscape("--output=pwned2.txt")+"/", token, nil)
	if resp.StatusCode != 400 {
		t.Fatalf("commit detail with option sha = %d, want 400", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "alice", "opts.git", "pwned2.txt")); err == nil {
		t.Fatalf("git wrote --output file into the repo dir")
	}

	// a plain ref must keep working
	resp, b = authReq("GET", base+"/api/v1/projects/1/commits/?ref=main", token, nil)
	if resp.StatusCode != 200 || !strings.Contains(string(b), "results") {
		t.Fatalf("commits with normal ref = %d (%s)", resp.StatusCode, b)
	}
}

// TestGuestCannotMutateWebhooks: hooks are repository configuration — a guest
// (role 10) must not create or delete them (previously: could, including
// global webhooks).
func TestGuestCannotMutateWebhooks(t *testing.T) {
	app, base, _ := newTestApp(t)
	alice := registerAndLogin(t, base)
	authReq("POST", base+"/api/v1/projects/", alice, map[string]any{"name": "hooks-authz", "visibility": "public"})
	authReq("POST", base+"/api/v1/projects/1/hooks/", alice, map[string]any{"url": "https://example.com/hook"})

	http.Post(base+"/api/v1/auth/register/", "application/json",
		strings.NewReader(`{"username":"webhookbob","email":"webhookbob@example.com","password":"password123"}`))
	var login struct {
		Access string `json:"access"`
	}
	respLogin, err := http.Post(base+"/api/v1/auth/login/", "application/json",
		strings.NewReader(`{"username":"webhookbob","password":"password123"}`))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer respLogin.Body.Close()
	_ = json.NewDecoder(respLogin.Body).Decode(&login)
	bob := login.Access

	bobUser, _ := app.Store.GetUserByUsername("webhookbob")
	if bobUser == nil {
		t.Fatalf("webhookbob not found")
	}
	if err := app.Store.SetAccess(bobUser.ID, 1, 10); err != nil {
		t.Fatalf("grant guest: %v", err)
	}

	resp, b := authReq("POST", base+"/api/v1/projects/1/hooks/", bob, map[string]any{"url": "https://example.com/evil"})
	if resp.StatusCode != 403 {
		t.Fatalf("guest create webhook = %d (%s), want 403", resp.StatusCode, b)
	}
	resp, b = authReq("DELETE", base+"/api/v1/projects/1/hooks/1/", bob, nil)
	if resp.StatusCode != 403 {
		t.Fatalf("guest delete webhook = %d (%s), want 403", resp.StatusCode, b)
	}
}

// TestUsernameRenameMigratesRepos: renaming a user must rewrite stored repo
// paths and move the bare directories, otherwise every repo of that user
// becomes unreachable at the old URL and unfindable at the new one.
func TestUsernameRenameMigratesRepos(t *testing.T) {
	_, base, repoRoot := newTestApp(t)
	alice := registerAndLogin(t, base) // first user = superuser
	authReq("POST", base+"/api/v1/projects/", alice, map[string]any{"name": "migrate"})
	pushFile(t, filepath.Join(repoRoot, "alice", "migrate.git"), "a.txt", "x")

	resp, b := authReq("PATCH", base+"/api/v1/users/alice/", alice, map[string]any{"username": "alice2"})
	if resp.StatusCode != 200 {
		t.Fatalf("rename = %d (%s)", resp.StatusCode, b)
	}

	resp, _ = authReq("GET", base+"/api/v1/projects/by-path/alice2/migrate/", alice, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("repo at new username = %d, want 200", resp.StatusCode)
	}
	resp, _ = authReq("GET", base+"/api/v1/projects/by-path/alice/migrate/", alice, nil)
	if resp.StatusCode != 404 {
		t.Fatalf("repo at old username = %d, want 404", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "alice2", "migrate.git", "HEAD")); err != nil {
		t.Fatalf("repo dir not moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "alice", "migrate.git")); !os.IsNotExist(err) {
		t.Fatalf("old repo dir still exists")
	}
}

// TestCustomDiskPathOutsideRootRejected: custom storage paths are bounded by
// MYGIT_CUSTOM_REPOS_ROOT so neither creation nor deletion can touch
// arbitrary locations on disk.
func TestCustomDiskPathOutsideRootRejected(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)
	outside := filepath.Join(t.TempDir(), "customrepo")
	resp, b := authReq("POST", base+"/api/v1/projects/", token, map[string]any{
		"name": "outside", "custom_disk_path": outside,
	})
	if resp.StatusCode != 400 {
		t.Fatalf("custom path outside root = %d (%s), want 400", resp.StatusCode, b)
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("directory was created outside the custom root")
	}
}

// TestDeleteRepoCascadesComments: deleting a repository must not leave
// orphaned comment rows behind.
func TestDeleteRepoCascadesComments(t *testing.T) {
	app, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)
	authReq("POST", base+"/api/v1/projects/", token, map[string]any{"name": "cascade"})
	authReq("POST", base+"/api/v1/projects/1/issues/", token, map[string]any{"title": "t", "description": "d"})
	authReq("POST", base+"/api/v1/projects/1/issues/1/comments/", token, map[string]any{"body": "c"})

	resp, _ := authReq("DELETE", base+"/api/v1/projects/1/", token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	var n int
	if err := app.Store.DB.QueryRow(`SELECT COUNT(*) FROM issue_comments`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("orphaned issue_comments rows: %d", n)
	}
}

// TestGitRPCErrorReturns500: a git process that dies before producing pack
// data must yield HTTP 500, not a 200 with a truncated pack body.
func TestGitRPCErrorReturns500(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)
	authReq("POST", base+"/api/v1/projects/", token, map[string]any{"name": "rpc500"})

	req, _ := http.NewRequest("POST", base+"/alice/rpc500/git-upload-pack", strings.NewReader("garbage-not-pkt-line"))
	req.SetBasicAuth("alice", "password123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("rpc: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 500 {
		t.Fatalf("git-upload-pack with garbage body = %d (%s), want 500", resp.StatusCode, body)
	}
}

// TestCloneURLBlocksLoopback: the server makes outbound requests to custom
// import URLs — loopback/link-local targets (cloud metadata!) must be refused.
func TestCloneURLBlocksLoopback(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)
	for _, u := range []string{
		"http://127.0.0.1:8060/x.git",
		"http://169.254.169.254/latest/x.git",
		"http://localhost/x.git",
	} {
		resp, b := authReq("POST", base+"/api/v1/projects/import/", token, map[string]any{
			"provider": "custom", "name": "imp", "clone_url": u,
		})
		if resp.StatusCode != 400 {
			t.Fatalf("clone_url %s = %d (%s), want 400", u, resp.StatusCode, b)
		}
	}
}

// TestRateLimiterUnit: fixed-window limiter refuses the (limit+1)-th request
// from the same IP without affecting other IPs.
func TestRateLimiterUnit(t *testing.T) {
	rl := newRateLimiter(2, time.Minute)
	if !rl.allow("1.2.3.4") || !rl.allow("1.2.3.4") {
		t.Fatalf("first two requests must pass")
	}
	if rl.allow("1.2.3.4") {
		t.Fatalf("third request within the window must be limited")
	}
	if !rl.allow("5.6.7.8") {
		t.Fatalf("another IP must be unaffected")
	}
}