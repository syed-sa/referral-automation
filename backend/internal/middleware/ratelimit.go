package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"referralautomation/internal/auth"
	"referralautomation/internal/platform/apperr"
	"referralautomation/internal/platform/httpx"
)

const (
	limiterIdleTTL     = 10 * time.Minute
	limiterSweepEvery  = 1000 // requests between sweeps of idle entries
	limiterMaxTracking = 50_000
)

type limiterEntry struct {
	lim  *rate.Limiter
	last time.Time
}

// RateLimiter is a per-user token bucket kept in memory (single instance; this
// app deliberately has no Redis). Idle entries are swept lazily on requests, so
// no background goroutine is needed.
type RateLimiter struct {
	mu    sync.Mutex
	items map[string]*limiterEntry
	rps   rate.Limit
	burst int
	calls int
}

func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	return &RateLimiter{items: map[string]*limiterEntry{}, rps: rate.Limit(requestsPerSecond), burst: burst}
}

func (rl *RateLimiter) allow(key string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.calls++
	if rl.calls%limiterSweepEvery == 0 || len(rl.items) > limiterMaxTracking {
		for k, e := range rl.items {
			if now.Sub(e.last) > limiterIdleTTL {
				delete(rl.items, k)
			}
		}
	}
	e, ok := rl.items[key]
	if !ok {
		e = &limiterEntry{lim: rate.NewLimiter(rl.rps, rl.burst)}
		rl.items[key] = e
	}
	e.last = now
	return e.lim.Allow()
}

// PerUser limits by the authenticated user id; it must run after Authenticate.
func (rl *RateLimiter) PerUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFrom(r.Context())
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if !rl.allow(p.UserID.String()) {
			w.Header().Set("Retry-After", "1")
			httpx.WriteError(w, r, apperr.New(http.StatusTooManyRequests, "rate_limited", "too many requests, slow down"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
