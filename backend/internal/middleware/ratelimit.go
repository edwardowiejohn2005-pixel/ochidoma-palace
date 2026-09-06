// Package middleware also provides a basic in-memory per-IP rate limiter.
// This is fine for a single-instance deployment; if you later scale the API
// horizontally behind a load balancer, move this to Redis so limits are
// shared across instances instead of reset per-process.
package middleware

import (
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	b        int
}

func newIPLimiter(requestsPerMinute float64, burst int) *ipLimiter {
	return &ipLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        rate.Limit(requestsPerMinute / 60),
		b:        burst,
	}
}

func (l *ipLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.limiters[ip]
	if !ok {
		lim = rate.NewLimiter(l.r, l.b)
		l.limiters[ip] = lim
	}
	return lim
}

// RateLimitByIP allows requestsPerMinute sustained, up to burst at once, per
// client IP. Intended for sensitive low-traffic routes like /api/auth/login —
// not for general API traffic.
func RateLimitByIP(requestsPerMinute float64, burst int) func(http.Handler) http.Handler {
	limiter := newIPLimiter(requestsPerMinute, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !limiter.get(ip).Allow() {
				http.Error(w, `{"error":"too many attempts, please wait and try again"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
