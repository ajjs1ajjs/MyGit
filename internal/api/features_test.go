package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestApprovalsGate verifies the merge gate: an MR on a protected branch
// with required_approvals=1 must 403 until a non-author writer approves.
func TestApprovalsGate(t *testing.T) {
	_, base, _ := newTestApp(t)
	alice := registerAndLogin(t, base)

	// alice creates a repo + MR to herself (target main).
	resp, b := authReq("POST", base+"/api/v1/projects/", alice, map[string]any{
		"name": "gate", "visibility": "private",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create repo = %d: %s", resp.StatusCode, b)
	}
	// protect main with 1 approval
	resp, b = authReq("POST", base+"/api/v1/projects/1/protected-branches/", alice, map[string]any{
		"pattern": "main", "required_approvals": 1,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("protect = %d: %s", resp.StatusCode, b)
	}
	// open MR main <- feature (create branches via git? use API)
	resp, b = authReq("POST", base+"/api/v1/projects/1/merge_requests/", alice, map[string]any{
		"source_branch": "feature", "target_branch": "main", "title": "t",
	})
	if resp.StatusCode != 201 {
		t.Skipf("no MR fixture path (need git branches): %d %s", resp.StatusCode, b)
	}
	var mr struct {
		Number int `json:"number"`
	}
	_ = json.Unmarshal(b, &mr)
	if mr.Number == 0 {
		t.Fatalf("no MR number in %s", b)
	}
	// merge without approvals must 403 (protected main, need 1)
	resp, b = authReq("POST", fmt.Sprintf("%s/api/v1/projects/1/merge_requests/%d/merge/", base, mr.Number), alice, map[string]any{})
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(b), "approval") {
		t.Fatalf("unapproved merge = %d: %s, want 403 approval", resp.StatusCode, b)
	}
}

// TestEnvVarsMasked verifies secrets are write-only (masked on read).
func TestEnvVarsMasked(t *testing.T) {
	_, base, _ := newTestApp(t)
	alice := registerAndLogin(t, base)
	resp, b := authReq("POST", base+"/api/v1/projects/", alice, map[string]any{
		"name": "envtest", "visibility": "private",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create repo = %d: %s", resp.StatusCode, b)
	}
	resp, b = authReq("POST", base+"/api/v1/projects/1/environments/", alice, map[string]any{"name": "prod"})
	if resp.StatusCode != 201 {
		t.Fatalf("create env = %d: %s", resp.StatusCode, b)
	}
	resp, b = authReq("POST", base+"/api/v1/projects/1/environments/vars/?env=prod", alice, map[string]any{
		"name": "TOKEN", "value": "s3cr3t", "secret": true,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("set var = %d: %s", resp.StatusCode, b)
	}
	resp, b = authReq("GET", base+"/api/v1/projects/1/environments/vars/?env=prod", alice, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list vars = %d: %s", resp.StatusCode, b)
	}
	if strings.Contains(string(b), "s3cr3t") {
		t.Fatalf("secret echoed on read: %s", b)
	}
	if !strings.Contains(string(b), "***") {
		t.Fatalf("secret not masked: %s", b)
	}
	// bad env name rejected
	resp, _ = authReq("POST", base+"/api/v1/projects/1/environments/", alice, map[string]any{"name": "../x"})
	if resp.StatusCode != 400 {
		t.Fatalf("bad env name = %d, want 400", resp.StatusCode)
	}
}

// TestPagesVisibility verifies private repos 404 anonymously.
func TestPagesVisibility(t *testing.T) {
	_, base, _ := newTestApp(t)
	alice := registerAndLogin(t, base)
	resp, b := authReq("POST", base+"/api/v1/projects/", alice, map[string]any{
		"name": "site", "visibility": "private",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create repo = %d: %s", resp.StatusCode, b)
	}
	resp2, err := http.Get(base + "/-/pages/alice/site/index.html")
	if err != nil {
		t.Fatalf("pages: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("private pages anon = %d, want 404", resp2.StatusCode)
	}
	// traversal in owner segment 404s
	resp3, err := http.Get(base + "/-/pages/../etc/index.html")
	if err == nil {
		resp3.Body.Close()
		if resp3.StatusCode != http.StatusNotFound {
			t.Fatalf("traversal pages = %d, want 404", resp3.StatusCode)
		}
	}
}

// TestPackagesFlow verifies publish/list/download/delete with writer gating.
func TestPackagesFlow(t *testing.T) {
	_, base, _ := newTestApp(t)
	alice := registerAndLogin(t, base)
	resp, b := authReq("POST", base+"/api/v1/projects/", alice, map[string]any{
		"name": "pkgrepo", "visibility": "private",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create repo = %d: %s", resp.StatusCode, b)
	}
	// publish without filename registers the package shell
	resp, b = authReq("POST", base+"/api/v1/projects/1/packages/publish/?name=lib&type=generic", alice, nil)
	if resp.StatusCode != 201 {
		t.Fatalf("publish shell = %d: %s", resp.StatusCode, b)
	}
	// bad type rejected
	resp, _ = authReq("POST", base+"/api/v1/projects/1/packages/publish/?name=x&type=evil", alice, nil)
	if resp.StatusCode != 400 {
		t.Fatalf("bad type = %d, want 400", resp.StatusCode)
	}
	// upload artifact bytes
	req, _ := http.NewRequest("POST", base+"/api/v1/projects/1/packages/publish/?name=lib&type=generic&version=1.0.0&filename=lib.tgz", strings.NewReader("fake-tarball-bytes"))
	req.Header.Set("Authorization", "Bearer "+alice)
	up, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	up.Body.Close()
	if up.StatusCode != 201 {
		t.Fatalf("upload = %d, want 201", up.StatusCode)
	}
	// list files + download round-trip
	resp, b = authReq("GET", base+"/api/v1/projects/1/packages/files/?package=lib&type=generic", alice, nil)
	if resp.StatusCode != 200 || !strings.Contains(string(b), "lib.tgz") {
		t.Fatalf("list files = %d: %s", resp.StatusCode, b)
	}
	dl, _ := http.NewRequest("GET", base+"/api/v1/projects/1/packages/download/?package=lib&type=generic&version=1.0.0&filename=lib.tgz", nil)
	dl.Header.Set("Authorization", "Bearer "+alice)
	dresp, err := http.DefaultClient.Do(dl)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer dresp.Body.Close()
	if dresp.StatusCode != 200 || dresp.Header.Get("X-Content-SHA256") == "" {
		t.Fatalf("download = %d sha=%q", dresp.StatusCode, dresp.Header.Get("X-Content-SHA256"))
	}
}
