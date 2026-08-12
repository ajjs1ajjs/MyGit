package api

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ajjs1ajjs/MyGit/internal/auth"
	"github.com/ajjs1ajjs/MyGit/internal/config"
	"github.com/ajjs1ajjs/MyGit/internal/git"
	"github.com/ajjs1ajjs/MyGit/internal/storage"
	"github.com/go-chi/chi/v5"
)

//go:embed all:web
var webFS embed.FS

type App struct {
	Cfg   *config.Config
	Store *storage.Store
	Auth  *auth.Auth
	Git   *git.Backend
	Start time.Time
}

type principal struct {
	UserID   int64
	Username string
	IsSuper  bool
	Method   string // jwt | pat | basic | internal
}

type ctxKey int

const principalKey ctxKey = 0

func (a *App) Handler() http.Handler {
	r := chi.NewRouter()

	// smart git HTTP
	r.Get("/{owner}/{repo}/info/refs", a.handleGitInfoRefs)
	r.Post("/{owner}/{repo}/git-upload-pack", a.handleGitRPC)
	r.Post("/{owner}/{repo}/git-receive-pack", a.handleGitRPC)

	// liveness/health (unauthenticated) — used by installers/uptime checks to
	// verify THIS service is up, not whatever else happens to be on the port.
	r.Get("/api/v1/health", a.handleHealth)

	// internal (git hooks / CI runner)
	r.With(a.withInternal).Post("/api/v1/internal/pre-receive", a.handlePreReceive)
	r.With(a.withInternal).Post("/api/v1/internal/post-receive", a.handlePostReceive)
	r.With(a.withInternal).Post("/api/v1/internal/check_access", a.handleCheckAccess)
	r.With(a.withInternal).Get("/api/v1/internal/authorized_keys", a.handleAuthorizedKeys)

	// auth (rate-limited to slow down brute force / credential stuffing)
	authLimiter := newRateLimiter(10, time.Minute)
	r.With(a.withRateLimit(authLimiter)).Post("/api/v1/auth/register/", a.handleRegister)
	r.With(a.withRateLimit(authLimiter)).Post("/api/v1/auth/login/", a.handleLogin)
	r.With(a.withRateLimit(authLimiter)).Post("/api/v1/auth/refresh/", a.handleRefresh)
	r.Post("/api/v1/auth/logout/", a.handleLogout)

	// users
	r.Route("/api/v1/users", func(r chi.Router) {
		r.With(a.withAuth).Get("/me/", a.handleMe)
		r.With(a.withAuth).Patch("/me/", a.handleUpdateMe)
		r.With(a.withAuth).Post("/change_password/", a.handleChangePassword)
		r.With(a.withAuth).Get("/{username}/keys/", a.handleListKeys)
		r.With(a.withAuth).Post("/{username}/keys/", a.handleAddKey)
		r.With(a.withAuth).Delete("/{username}/keys/{keyID}/", a.handleDeleteKey)
		r.With(a.withAuth).Get("/{username}/tokens/", a.handleListTokens)
		r.With(a.withAuth).Post("/{username}/tokens/", a.handleCreateToken)
		r.With(a.withAuth).Delete("/{username}/tokens/{tokenID}/", a.handleDeleteToken)
	})

	// projects
	r.Route("/api/v1/projects", func(r chi.Router) {
		r.With(a.withAuth).Get("/", a.handleListProjects)
		r.With(a.withAuth).Post("/", a.handleCreateProject)
		r.Get("/by-path/{owner}/{repo}/", a.handleProjectByPath)
		r.With(a.withAuth).Post("/{id}/fork/", a.handleForkProject)
		r.With(a.withAuth).Get("/{id}/tree/", a.handleTree)
		r.With(a.withAuth).Get("/{id}/raw/", a.handleRaw)
		r.With(a.withAuth).Get("/{id}/blobs/{sha}/", a.handleBlob)
		r.With(a.withAuth).Get("/{id}/blame/", a.handleBlame)
		r.With(a.withAuth).Get("/{id}/commits/", a.handleCommits)
		r.With(a.withAuth).Get("/{id}/commits/{sha}/", a.handleCommitDetail)
		r.With(a.withAuth).Get("/{id}/commits/{sha}/diff/", a.handleCommitDiff)
		r.With(a.withAuth).Get("/{id}/branches/", a.handleBranches)
		r.With(a.withAuth).Post("/{id}/branches/", a.handleCreateBranch)
		r.With(a.withAuth).Delete("/{id}/branches/{name}/", a.handleDeleteBranch)
		r.With(a.withAuth).Get("/{id}/tags/", a.handleTags)
		r.With(a.withAuth).Get("/{id}/", a.handleGetProject)
		r.With(a.withAuth).Patch("/{id}/", a.handleUpdateProject)
		r.With(a.withAuth).Delete("/{id}/", a.handleDeleteProject)
		r.With(a.withAuth).Get("/{id}/issues/", a.handleListIssues)
		r.With(a.withAuth).Post("/{id}/issues/", a.handleCreateIssue)
		r.With(a.withAuth).Get("/{id}/issues/{number}/", a.handleGetIssue)
		r.With(a.withAuth).Patch("/{id}/issues/{number}/", a.handleUpdateIssue)
		r.With(a.withAuth).Get("/{id}/issues/{number}/comments/", a.handleListIssueComments)
		r.With(a.withAuth).Post("/{id}/issues/{number}/comments/", a.handleAddIssueComment)
		r.With(a.withAuth).Get("/{id}/merge_requests/", a.handleListMRs)
		r.With(a.withAuth).Post("/{id}/merge_requests/", a.handleCreateMR)
		r.With(a.withAuth).Get("/{id}/merge_requests/{number}/", a.handleGetMR)
		r.With(a.withAuth).Post("/{id}/merge_requests/{number}/merge/", a.handleMergeMR)
		r.With(a.withAuth).Get("/{id}/merge_requests/{number}/comments/", a.handleListMRComments)
		r.With(a.withAuth).Post("/{id}/merge_requests/{number}/comments/", a.handleAddMRComment)
		r.With(a.withAuth).Get("/{id}/merge_requests/{number}/diff/", a.handleMRDiff)
		r.With(a.withAuth).Get("/{id}/hooks/", a.handleListWebhooks)
		r.With(a.withAuth).Post("/{id}/hooks/", a.handleCreateWebhook)
		r.With(a.withAuth).Delete("/{id}/hooks/{hookID}/", a.handleDeleteWebhook)
		r.With(a.withAuth).Get("/{id}/wiki/", a.handleListWiki)
		r.With(a.withAuth).Post("/{id}/wiki/", a.handleCreateWiki)
		r.With(a.withAuth).Put("/{id}/wiki/{slug}/", a.handleUpdateWiki)
	})

	// notifications / search
	r.With(a.withAuth).Get("/api/v1/notifications/", a.handleNotifications)
	r.With(a.withAuth).Get("/api/v1/search/", a.handleSearch)

	// SPA (Vue history mode): /assets/* and index.html fallback
	web, _ := fs.Sub(webFS, "web")
	r.Get("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.FS(web))).ServeHTTP)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		// Never hand out the SPA HTML for unmatched API paths — a client that
		// calls an unknown /api/ endpoint must get JSON, otherwise the frontend
		// fails with "Unexpected token '<'" when it tries to parse index.html.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeErr(w, http.StatusNotFound, "Not found")
			return
		}
		a.spaHandler(web).ServeHTTP(w, r)
	})

	return a.withSecurity(a.withLogging(r))
}

// spaHandler serves index.html for any non-API, non-git GET route.
func (a *App) spaHandler(web fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		b, err := fs.ReadFile(web, "index.html")
		if err != nil {
			writeErr(w, http.StatusNotFound, "frontend not built")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	}
}

// --- middleware ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr uses the DRF-style "detail" field (a lesson from the other ports).
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"detail": msg})
}

// handleHealth reports that this MyGit process is up (used by installers and
// external uptime checks). It is unauthenticated on purpose.
func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// authenticate resolves a principal from: Bearer JWT (access), Bearer PAT,
// or HTTP Basic (username + password/PAT).
func (a *App) authenticate(r *http.Request) (*principal, error) {
	authz := r.Header.Get("Authorization")
	if strings.HasPrefix(authz, "Bearer ") {
		token := strings.TrimPrefix(authz, "Bearer ")
		if claims, err := a.Auth.Parse(token, "access"); err == nil {
			// Reject tokens whose token_version is stale (e.g. after a password
			// change), giving us server-side JWT revocation.
			u, _ := a.Store.GetUserByID(claims.UserID)
			if u == nil || u.IsActive != 1 {
				return nil, errors.New("user not found")
			}
			if claims.Ver != int64(u.TokenVersion) {
				return nil, errors.New("token has been revoked")
			}
			return &principal{UserID: claims.UserID, Username: claims.Username, IsSuper: u.IsSuperuser == 1, Method: "jwt"}, nil
		}
		if pat, err := a.Store.GetTokenByHash(auth.HashToken(token)); err == nil && pat != nil {
			a.Store.TouchToken(pat.ID)
			u, _ := a.Store.GetUserByID(pat.UserID)
			if u != nil {
				return &principal{UserID: u.ID, Username: u.Username, IsSuper: u.IsSuperuser == 1, Method: "pat"}, nil
			}
		}
		return nil, errors.New("invalid token")
	}
	if user, pass, ok := r.BasicAuth(); ok {
		u, err := a.Store.GetUserByUsername(user)
		if err != nil || u == nil {
			u, _ = a.Store.GetUserByEmail(user)
		}
		if u != nil {
			if auth.VerifyPassword(u.PasswordHash, pass) {
				return &principal{UserID: u.ID, Username: u.Username, IsSuper: u.IsSuperuser == 1, Method: "basic"}, nil
			}
			if pat, err := a.Store.GetTokenByHash(auth.HashToken(pass)); err == nil && pat != nil && pat.UserID == u.ID {
				a.Store.TouchToken(pat.ID)
				return &principal{UserID: u.ID, Username: u.Username, IsSuper: u.IsSuperuser == 1, Method: "basic"}, nil
			}
		}
		return nil, errors.New("invalid credentials")
	}
	// HttpOnly session cookie (SPA). Checked last so programmatic clients that
	// send an explicit Authorization header take precedence.
	if c, err := r.Cookie(cookieAccess); err == nil {
		if claims, err := a.Auth.Parse(c.Value, "access"); err == nil {
			u, _ := a.Store.GetUserByID(claims.UserID)
			if u != nil && u.IsActive == 1 && claims.Ver == int64(u.TokenVersion) {
				return &principal{UserID: u.ID, Username: u.Username, IsSuper: u.IsSuperuser == 1, Method: "cookie"}, nil
			}
		}
	}
	return nil, errors.New("not authenticated")
}

func (a *App) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := a.authenticate(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, p)))
	})
}

func (a *App) withInternal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") || !constantTimeEqual(strings.TrimPrefix(authz, "Bearer "), a.Cfg.InternalToken) {
			writeErr(w, http.StatusForbidden, "Invalid internal token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// constantTimeEqual compares two strings in constant time to avoid leaking the
// internal token length/prefix via timing.
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func (a *App) principal(r *http.Request) *principal {
	if v, ok := r.Context().Value(principalKey).(*principal); ok {
		return v
	}
	return nil
}

func (a *App) withSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// HSTS only applies when TLS is terminated by this process.
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		// Vue injects some styles via inline style attributes; scripts are fully
		// bundled so 'self' is sufficient for script-src.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; font-src 'self' data:; connect-src 'self'; "+
				"frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func (a *App) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Hijack lets git smart HTTP (and WebSocket) upgrade through the logging wrapper.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hj.Hijack()
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(w.ResponseWriter, r)
}
