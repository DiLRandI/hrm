package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"hrm/internal/transport/http/api"
)

type RateLimitKeyFunc func(r *http.Request) string

type RateLimitOption func(*rateLimiter)

type rateBucket struct {
	count int
	reset time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	keyFn   RateLimitKeyFunc
	clients map[string]*rateBucket
}

func WithKeyFunc(fn RateLimitKeyFunc) RateLimitOption {
	return func(rl *rateLimiter) {
		if fn != nil {
			rl.keyFn = fn
		}
	}
}

func RateLimit(limit int, window time.Duration, opts ...RateLimitOption) func(http.Handler) http.Handler {
	rl := newRateLimiter(limit, window, actorOrIPKey)
	for _, opt := range opts {
		opt(rl)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.enforce(w, r) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func SensitiveMutationRateLimit(baseLimit int, window time.Duration) func(http.Handler) http.Handler {
	authLimit := max(baseLimit/4, 1)
	mutationLimit := max(baseLimit/2, 1)
	authByIP := newRateLimiter(authLimit, window, clientIPKey)
	authByEmail := newRateLimiter(authLimit, window, AuthEmailOrIPKey("email"))
	sensitiveByActor := newRateLimiter(mutationLimit, window, actorOrIPKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scope := sensitiveRateScope(r)
			switch scope {
			case sensitiveScopeAuth:
				if !authByIP.enforce(w, r) {
					return
				}
				if !authByEmail.enforce(w, r) {
					return
				}
			case sensitiveScopeActor:
				if !sensitiveByActor.enforce(w, r) {
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func AuthEmailOrIPKey(field string) RateLimitKeyFunc {
	normalizedField := strings.TrimSpace(field)
	if normalizedField == "" {
		normalizedField = "email"
	}
	return func(r *http.Request) string {
		email := extractJSONField(r, normalizedField)
		if email == "" {
			return clientIPKey(r)
		}
		return "email:" + strings.ToLower(email)
	}
}

func clientKey(r *http.Request) string {
	return clientIPKey(r)
}

func actorOrIPKey(r *http.Request) string {
	if user, ok := GetUser(r.Context()); ok && user.UserID != "" {
		return "user:" + user.TenantID + ":" + user.UserID
	}
	return clientIPKey(r)
}

func clientIPKey(r *http.Request) string {
	if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); fwd != "" {
		parts := strings.Split(fwd, ",")
		if len(parts) > 0 {
			value := strings.TrimSpace(parts[0])
			if value != "" {
				return value
			}
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func newRateLimiter(limit int, window time.Duration, keyFn RateLimitKeyFunc) *rateLimiter {
	if keyFn == nil {
		keyFn = actorOrIPKey
	}
	return &rateLimiter{
		limit:   limit,
		window:  window,
		keyFn:   keyFn,
		clients: map[string]*rateBucket{},
	}
}

func (rl *rateLimiter) enforce(w http.ResponseWriter, r *http.Request) bool {
	if rl.limit <= 0 {
		return true
	}

	key := rl.keyFn(r)
	if key == "" {
		key = clientIPKey(r)
	}
	now := time.Now()

	rl.mu.Lock()
	bucket, ok := rl.clients[key]
	if !ok || now.After(bucket.reset) {
		bucket = &rateBucket{count: 0, reset: now.Add(rl.window)}
		rl.clients[key] = bucket
	}
	bucket.count++
	remaining := rl.limit - bucket.count
	resetIn := durationSeconds(bucket.reset.Sub(now))
	overLimit := bucket.count > rl.limit
	rl.mu.Unlock()

	w.Header().Set("X-RateLimit-Limit", itoa(rl.limit))
	w.Header().Set("X-RateLimit-Remaining", itoa(max(remaining, 0)))
	w.Header().Set("X-RateLimit-Reset", itoa(resetIn))

	if overLimit {
		w.Header().Set("Retry-After", itoa(max(resetIn, 1)))
		slog.Warn("rate limit exceeded",
			"key", key,
			"path", r.URL.Path,
			"method", r.Method,
			"limit", rl.limit,
			"windowSec", int(rl.window.Seconds()),
		)
		api.Fail(w, http.StatusTooManyRequests, "rate_limited", "too many requests", GetRequestID(r.Context()))
		return false
	}

	return true
}

func durationSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	seconds := int(d.Seconds())
	if seconds <= 0 {
		return 1
	}
	return seconds
}

func extractJSONField(r *http.Request, field string) string {
	if r == nil || r.Body == nil {
		return ""
	}
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if !strings.Contains(contentType, "application/json") {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if len(raw) == 0 {
		return ""
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	value, _ := payload[field].(string)
	return strings.TrimSpace(value)
}

type sensitiveScope string

const (
	sensitiveScopeNone  sensitiveScope = ""
	sensitiveScopeAuth  sensitiveScope = "auth"
	sensitiveScopeActor sensitiveScope = "actor"
)

func sensitiveRateScope(r *http.Request) sensitiveScope {
	if r == nil {
		return sensitiveScopeNone
	}
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && method != http.MethodDelete {
		return sensitiveScopeNone
	}

	path := normalizedAPIPath(r.URL.Path)
	switch path {
	case "/auth/login",
		"/auth/request-reset",
		"/auth/reset",
		"/auth/mfa/setup",
		"/auth/mfa/enable",
		"/auth/mfa/disable":
		return sensitiveScopeAuth
	case "/leave/accrual/run",
		"/gdpr/retention/run",
		"/gdpr/dsar",
		"/gdpr/anonymize":
		return sensitiveScopeActor
	}

	if strings.HasPrefix(path, "/leave/requests/") && (strings.HasSuffix(path, "/approve") || strings.HasSuffix(path, "/reject")) {
		return sensitiveScopeActor
	}
	if strings.HasPrefix(path, "/payroll/periods/") && (strings.HasSuffix(path, "/run") || strings.HasSuffix(path, "/finalize") || strings.HasSuffix(path, "/inputs/import")) {
		return sensitiveScopeActor
	}
	if strings.HasPrefix(path, "/gdpr/anonymize/") && strings.HasSuffix(path, "/execute") {
		return sensitiveScopeActor
	}

	return sensitiveScopeNone
}

func normalizedAPIPath(path string) string {
	cleaned := strings.TrimSpace(path)
	if strings.HasPrefix(cleaned, "/api/v1") {
		cleaned = strings.TrimPrefix(cleaned, "/api/v1")
	}
	if cleaned == "" {
		return "/"
	}
	if !strings.HasPrefix(cleaned, "/") {
		return "/" + cleaned
	}
	return cleaned
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
