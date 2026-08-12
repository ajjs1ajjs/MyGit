package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ipBucket is a fixed-window counter for one client IP.
type ipBucket struct {
	count   int
	resetAt time.Time
}

// rateLimiter is a simple per-IP fixed-window limiter with periodic cleanup of
// expired buckets so the map cannot grow without bound.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*ipBucket
	limit   int
	window  time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*ipBucket),
		limit:   limit,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if now.After(b.resetAt) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok || now.After(b.resetAt) {
		rl.buckets[ip] = &ipBucket{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	b.count++
	return b.count <= rl.limit
}

// clientIP uses the socket peer address. X-Forwarded-For is only trusted when
// the instance runs behind a trusted reverse proxy (MYGIT_TRUST_PROXY=1), which
// is expected to overwrite the header so a client cannot spoof it. When not
// behind a proxy the first XFF entry would be attacker-controlled, so it is
// deliberately ignored.
func (a *App) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if a.Cfg.TrustProxy && r.Header.Get("X-Forwarded-For") != "" {
		// The proxy prepends the real client IP; take the first entry.
		first := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
		if first != "" {
			return first
		}
	}
	return host
}

func (a *App) withRateLimit(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(a.clientIP(r)) {
				w.Header().Set("Retry-After", "60")
				writeErr(w, http.StatusTooManyRequests, "Too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
