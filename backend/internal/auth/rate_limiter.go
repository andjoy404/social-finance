package auth

import (
	"net/http"
	"strings"
	"sync"
	"time"

	httpx "social-finance/internal/http"
)

// RateLimiter provides simple per-key in-memory rate limiting.
//
// IMPORTANT: rate limiting is per-backend-instance. In a horizontally scaled
// deployment (multiple replicas behind a load balancer), each instance has its
// own in-memory counters, meaning an attacker can spread requests across
// instances to exceed the limit. Shared-state backends (e.g. Redis) are needed
// for accurate multi-instance rate limiting. Redis is NOT added in Phase 2.
type RateLimiter struct {
	mu       sync.Mutex
	keyLimit map[string]limitInfo
	maxReq   int
	window   time.Duration
}

type limitInfo struct {
	count       int
	windowStart time.Time
}

// NewRateLimiter creates a rate limiter that allows maxReq requests
// per window duration per key.
func NewRateLimiter(maxReq int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		keyLimit: make(map[string]limitInfo),
		maxReq:   maxReq,
		window:   window,
	}
}

// Allow checks whether the given key is allowed to make a request.
// Returns false and blocks if the limit has been exceeded.
// If the key is not yet in the window, it is created with count=1.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	info, ok := rl.keyLimit[key]
	if !ok || now.Sub(info.windowStart) > rl.window {
		rl.keyLimit[key] = limitInfo{
			count:       1,
			windowStart: now,
		}
		return true
	}
	info.count++
	if info.count > rl.maxReq {
		return false
	}
	rl.keyLimit[key] = info
	return true
}

// LimitFunc returns an HTTP middleware that enforces rate limiting.
// When the limit is exceeded, sends 429 Too Many Requests using the standard
// API error format.
func (rl *RateLimiter) LimitFunc(keyFunc func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !rl.Allow(key) {
				httpx.ErrorJSON(w, http.StatusTooManyRequests, "rate_limited", "too many requests, please try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func stripPort(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return s[:i]
		}
	}
	return s
}

// ClientIP extracts the direct peer IP from the request, stripping the port.
// It uses r.RemoteAddr only — NOT X-Forwarded-For — so it cannot be spoofed
// by the client.  This is the correct rate-limit key for Phase 2's
// single-instance deployment.
func ClientIP(r *http.Request) string {
	return stripPort(r.RemoteAddr)
}

// clientIPWithProxy extracts the client IP using the trusted-proxy model.
// When the immediate connecting host (RemoteAddr) is in the trusted list,
// X-Forwarded-For is used. Otherwise RemoteAddr is used.
func clientIPWithProxy(r *http.Request, trusted []string) string {
	remote := r.RemoteAddr
	remoteIP := stripPort(remote)
	for _, t := range trusted {
		if t == remoteIP {
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				parts := strings.Split(fwd, ",")
				return strings.TrimSpace(parts[0])
			}
		}
	}
	return remoteIP
}

// LoginRateLimiter creates a limiter for the login endpoint: 10 attempts per
// 5 minutes per IP.
func LoginRateLimiter() *RateLimiter {
	return &RateLimiter{
		keyLimit: make(map[string]limitInfo),
		maxReq:   10,
		window:   5 * time.Minute,
	}
}

// RefreshRateLimiter creates a limiter for the refresh endpoint: 30 attempts
// per 15 minutes per IP.
func RefreshRateLimiter() *RateLimiter {
	return &RateLimiter{
		keyLimit: make(map[string]limitInfo),
		maxReq:   30,
		window:   15 * time.Minute,
	}
}
