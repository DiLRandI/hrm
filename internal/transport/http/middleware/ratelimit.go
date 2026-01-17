package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"hrm/internal/transport/http/api"
)

type rateBucket struct {
	count int
	reset time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]*rateBucket
}

func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		limit:   limit,
		window:  window,
		clients: map[string]*rateBucket{},
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			key := clientKey(r)
			now := time.Now()
			rl.mu.Lock()
			bucket, ok := rl.clients[key]
			if !ok || now.After(bucket.reset) {
				bucket = &rateBucket{count: 0, reset: now.Add(rl.window)}
				rl.clients[key] = bucket
			}
			bucket.count++
			remaining := rl.limit - bucket.count
			rl.mu.Unlock()

			w.Header().Set("X-RateLimit-Limit", itoa(rl.limit))
			w.Header().Set("X-RateLimit-Remaining", itoa(max(remaining, 0)))
			w.Header().Set("X-RateLimit-Reset", itoa(int(bucket.reset.Sub(now).Seconds())))

			if bucket.count > rl.limit {
				api.Fail(w, http.StatusTooManyRequests, "rate_limited", "too many requests", GetRequestID(r.Context()))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientKey(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
