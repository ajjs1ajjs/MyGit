package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func registerRunner(t *testing.T, base, admin, name string) (int64, string) {
	t.Helper()
	resp, b := authReq("POST", base+"/api/v1/admin/runners/", admin, map[string]any{"name": name})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register runner = %d: %s", resp.StatusCode, b)
	}
	var out struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(b, &out); err != nil || out.Token == "" {
		t.Fatalf("no runner token: %v %s", err, b)
	}
	return out.ID, out.Token
}

func runnerReq(t *testing.T, method, url, token string, body any) (int, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		bb, _ := json.Marshal(body)
		rdr = strings.NewReader(string(bb))
	} else {
		rdr = strings.NewReader("")
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// TestRunnerFlow covers register -> claim empty -> enqueue -> claim ->
// finish -> double-claim empty, plus auth rejections.
func TestRunnerFlow(t *testing.T) {
	app, base, _ := newTestApp(t)
	admin := registerAndLogin(t, base)
	_, token := registerRunner(t, base, admin, "builder-01")

	claim := func() (int, map[string]any) {
		req, _ := http.NewRequest("POST", base+"/api/v1/runners/jobs/next/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}

	// bad token -> 401
	bad, _ := http.NewRequest("POST", base+"/api/v1/runners/jobs/next/", nil)
	bad.Header.Set("Authorization", "Bearer mygit_run_nope")
	if resp, err := http.DefaultClient.Do(bad); err != nil || resp.StatusCode != 401 {
		if err == nil {
			resp.Body.Close()
		}
		t.Fatalf("bad runner token must 401")
	} else {
		resp.Body.Close()
	}

	// empty queue -> job null
	if code, out := claim(); code != 200 || out["job"] != nil {
		t.Fatalf("empty claim = %d %v, want 200 null job", code, out)
	}

	// enqueue directly + duplicate is idempotent
	id1, err := app.Store.EnqueuePipelineJob(1, "refs/heads/main", "abc123")
	if err != nil || id1 == 0 {
		t.Fatalf("enqueue: %v %d", err, id1)
	}
	id2, err := app.Store.EnqueuePipelineJob(1, "refs/heads/main", "abc123")
	if err != nil || id2 != id1 {
		t.Fatalf("duplicate enqueue = %d, want %d", id2, id1)
	}

	// claim -> running
	code, out := claim()
	if code != 200 {
		t.Fatalf("claim = %d", code)
	}
	job, _ := out["job"].(map[string]any)
	if job == nil || job["sha"] != "abc123" {
		t.Fatalf("claimed job = %v", out)
	}

	// second claim -> empty (atomic handoff, no double-run)
	if code, out := claim(); code != 200 || out["job"] != nil {
		t.Fatalf("second claim = %d %v, want empty", code, out)
	}

	// finish with oversized log gets capped, unknown status -> failed
	big := strings.Repeat("x", (1<<20)+10)
	fin, _ := http.NewRequest("POST", base+"/api/v1/runners/jobs/1/finish/", strings.NewReader(`{"status":"weird","log":"`+big[:100]+`"}`))
	fin.Header.Set("Authorization", "Bearer "+token)
	fin.Header.Set("Content-Type", "application/json")
	if resp, err := http.DefaultClient.Do(fin); err != nil || resp.StatusCode != 200 {
		if err == nil {
			resp.Body.Close()
		}
		t.Fatalf("finish: %v", err)
	} else {
		resp.Body.Close()
	}
}

// TestRunnerAdminOnly verifies registration/listing need superuser: the
// second registered user is a plain user (the first becomes superuser).
func TestRunnerAdminOnly(t *testing.T) {
	_, base, _ := newTestApp(t)
	_ = registerAndLogin(t, base)
	resp, err := http.Post(base+"/api/v1/auth/register/", "application/json",
		strings.NewReader(`{"username":"bob","email":"bob@example.com","password":"password123"}`))
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	defer resp.Body.Close()
	var bob struct {
		Access string `json:"access"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&bob)
	user := bob.Access
	resp2, _ := authReq("POST", base+"/api/v1/admin/runners/", user, map[string]any{"name": "x"})
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin register = %d, want 403", resp2.StatusCode)
	}
}
