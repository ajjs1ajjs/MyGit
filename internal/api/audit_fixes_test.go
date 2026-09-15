package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLogoutRevokesToken verifies single-session revocation: a Bearer token
// used on POST /logout is denied afterwards (was: valid until exp).
func TestLogoutRevokesToken(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)

	resp, _ := authReq("POST", base+"/api/v1/auth/logout/", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout = %d, want 200", resp.StatusCode)
	}
	resp, _ = authReq("GET", base+"/api/v1/users/me/", token, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused token after logout = %d, want 401", resp.StatusCode)
	}
}

// TestPasswordChangeInvalidatesOldTokens verifies the version bump: tokens
// issued before a password change die with the old password.
func TestPasswordChangeInvalidatesOldTokens(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)

	resp, b := authReq("POST", base+"/api/v1/users/change_password/", token, map[string]any{
		"current_password": "password123", "new_password": "NewStrongPass456",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("change-password = %d: %s", resp.StatusCode, b)
	}
	resp, _ = authReq("GET", base+"/api/v1/users/me/", token, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("pre-change token after rotation = %d, want 401", resp.StatusCode)
	}
}

// TestCookieCSRFGate verifies the double-submit gate on cookie sessions: a
// cross-site-shaped POST (no Origin/Referer, no token header) is 403, while
// the same request with a matching X-CSRF-Token passes the gate.
func TestCookieCSRFGate(t *testing.T) {
	_, base, _ := newTestApp(t)
	token := registerAndLogin(t, base)

	// Grab the session + CSRF cookies via a raw login.
	body := strings.NewReader(`{"username":"alice","password":"password123"}`)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/auth/login/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	_ = rec
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	cookies := map[string]string{}
	for _, c := range resp.Cookies() {
		cookies[c.Name] = c.Value
	}
	if cookies["mygit_csrf"] == "" {
		t.Fatalf("login did not issue a mygit_csrf cookie")
	}
	_ = token

	call := func(header string) int {
		r, _ := http.NewRequest(http.MethodPost, base+"/api/v1/auth/logout/", nil)
		for _, c := range resp.Cookies() {
			r.AddCookie(c)
		}
		if header != "" {
			r.Header.Set("X-CSRF-Token", header)
		}
		rr, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("req: %v", err)
		}
		defer rr.Body.Close()
		return rr.StatusCode
	}
	if code := call(""); code != http.StatusForbidden {
		t.Fatalf("cookie POST without CSRF evidence = %d, want 403", code)
	}
	if code := call("wrong"); code != http.StatusForbidden {
		t.Fatalf("cookie POST with wrong token = %d, want 403", code)
	}
	if code := call(cookies["mygit_csrf"]); code != http.StatusOK {
		t.Fatalf("cookie POST with matching token = %d, want 200", code)
	}
}

// TestAPIKeyReadOnly verifies scope: read-scoped keys can read but cannot
// change passwords or manage keys.
func TestAPIKeyReadOnly(t *testing.T) {
	_, base, _ := newTestApp(t)
	admin := registerAndLogin(t, base)

	resp, b := authReq("POST", base+"/api/v1/users/alice/tokens/", admin, map[string]any{"name": "scope-test", "scopes": []string{"read"}})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create key = %d: %s", resp.StatusCode, b)
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(b, &created); err != nil || created.Token == "" {
		t.Fatalf("no token returned: %v", err)
	}
	withKey := func(method, path string, body any) int {
		req, _ := http.NewRequest(method, base+path, nil)
		if body != nil {
			bb, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, base+path, strings.NewReader(string(bb)))
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+created.Token)
		rr, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("req: %v", err)
		}
		defer rr.Body.Close()
		return rr.StatusCode
	}
	if code := withKey(http.MethodGet, "/api/v1/users/me/", nil); code != http.StatusOK {
		t.Fatalf("key GET me = %d, want 200", code)
	}
	if code := withKey(http.MethodPost, "/api/v1/users/change_password/",
		map[string]any{"current_password": "x", "new_password": "NewStrongPass789"}); code != http.StatusForbidden {
		t.Fatalf("key change-password = %d, want 403", code)
	}
}

// TestUnknownScopeRejected verifies the creation allowlist: unknown scope
// strings are rejected instead of persisted.
func TestUnknownScopeRejected(t *testing.T) {
	_, base, _ := newTestApp(t)
	admin := registerAndLogin(t, base)
	resp, b := authReq("POST", base+"/api/v1/users/alice/tokens/", admin, map[string]any{"name": "x", "scopes": []string{"superpowers"}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown scope = %d: %s, want 400", resp.StatusCode, b)
	}
}

// TestCrossUserKeysReturn404 verifies the confused-deputy fix: bob's view of
// alice's key paths 404s instead of silently serving bob's own keys.
func TestCrossUserKeysReturn404(t *testing.T) {
	_, base, _ := newTestApp(t)
	alice := registerAndLogin(t, base)
	bobBody := `{"username":"bob","email":"bob@example.com","password":"password123"}`
	resp, err := http.Post(base+"/api/v1/auth/register/", "application/json", strings.NewReader(bobBody))
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	defer resp.Body.Close()
	var bob struct {
		Access string `json:"access"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&bob)
	for _, path := range []string{"/api/v1/users/alice/keys/", "/api/v1/users/alice/tokens/"} {
		r, _ := http.NewRequest(http.MethodGet, base+path, nil)
		r.Header.Set("Authorization", "Bearer "+bob.Access)
		rr, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("req: %v", err)
		}
		rr.Body.Close()
		if rr.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s as bob = %d, want 404", path, rr.StatusCode)
		}
	}
	_ = alice
}
