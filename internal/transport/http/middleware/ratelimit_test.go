package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hrm/internal/domain/auth"
)

func TestRateLimitUsesUserKeyBeforeIPFallback(t *testing.T) {
	limited := RateLimit(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	userCtx := context.WithValue(context.Background(), ctxKeyUser, auth.UserContext{
		TenantID: "tenant-1",
		UserID:   "user-1",
	})

	first := httptest.NewRequest(http.MethodPost, "/api/v1/payroll/periods/p1/finalize", nil).WithContext(userCtx)
	first.RemoteAddr = "198.51.100.11:2222"
	firstRec := httptest.NewRecorder()
	limited.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusNoContent {
		t.Fatalf("expected first request to pass, got %d", firstRec.Code)
	}

	second := httptest.NewRequest(http.MethodPost, "/api/v1/payroll/periods/p1/finalize", nil).WithContext(userCtx)
	second.RemoteAddr = "198.51.100.12:3333"
	secondRec := httptest.NewRecorder()
	limited.ServeHTTP(secondRec, second)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be throttled by user key, got %d", secondRec.Code)
	}
}

func TestRateLimitFallsBackToIP(t *testing.T) {
	limited := RateLimit(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRequest(http.MethodPost, "/api/v1/auth/request-reset", bytes.NewBufferString(`{"email":"a@example.com"}`))
	first.Header.Set("Content-Type", "application/json")
	first.RemoteAddr = "203.0.113.10:4444"
	firstRec := httptest.NewRecorder()
	limited.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusNoContent {
		t.Fatalf("expected first request to pass, got %d", firstRec.Code)
	}

	second := httptest.NewRequest(http.MethodPost, "/api/v1/auth/request-reset", bytes.NewBufferString(`{"email":"b@example.com"}`))
	second.Header.Set("Content-Type", "application/json")
	second.RemoteAddr = "203.0.113.10:5555"
	secondRec := httptest.NewRecorder()
	limited.ServeHTTP(secondRec, second)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be throttled by ip key, got %d", secondRec.Code)
	}
}

func TestRateLimitWindowReset(t *testing.T) {
	limited := RateLimit(1, 40*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"a@example.com"}`))
	req1.Header.Set("Content-Type", "application/json")
	req1.RemoteAddr = "192.0.2.20:1111"
	rec1 := httptest.NewRecorder()
	limited.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusNoContent {
		t.Fatalf("expected first request to pass, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"a@example.com"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.RemoteAddr = "192.0.2.20:1111"
	rec2 := httptest.NewRecorder()
	limited.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be throttled, got %d", rec2.Code)
	}

	time.Sleep(50 * time.Millisecond)

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"a@example.com"}`))
	req3.Header.Set("Content-Type", "application/json")
	req3.RemoteAddr = "192.0.2.20:1111"
	rec3 := httptest.NewRecorder()
	limited.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusNoContent {
		t.Fatalf("expected third request after window reset to pass, got %d", rec3.Code)
	}
}

func TestRateLimitReturnsRetryMetadata(t *testing.T) {
	limited := RateLimit(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"a@example.com"}`))
	req1.Header.Set("Content-Type", "application/json")
	req1.RemoteAddr = "192.0.2.30:1234"
	limited.ServeHTTP(httptest.NewRecorder(), req1)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"a@example.com"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.RemoteAddr = "192.0.2.30:1234"
	rec := httptest.NewRecorder()
	limited.ServeHTTP(rec, req2)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected throttled response, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
	if rec.Header().Get("X-RateLimit-Reset") == "" {
		t.Fatal("expected X-RateLimit-Reset header")
	}
}

func TestSensitiveMutationRateLimitScope(t *testing.T) {
	limited := SensitiveMutationRateLimit(4, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/dashboard/hr", nil)
		req.RemoteAddr = "198.51.100.40:8888"
		rec := httptest.NewRecorder()
		limited.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected read route request %d to bypass sensitive limits, got %d", i+1, rec.Code)
		}
	}

	userCtx := context.WithValue(context.Background(), ctxKeyUser, auth.UserContext{
		TenantID: "tenant-1",
		UserID:   "hr-1",
	})
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payroll/periods/p1/finalize", nil).WithContext(userCtx)
		req.RemoteAddr = "198.51.100.41:9999"
		rec := httptest.NewRecorder()
		limited.ServeHTTP(rec, req)
		if i < 2 && rec.Code != http.StatusNoContent {
			t.Fatalf("expected sensitive request %d to pass, got %d", i+1, rec.Code)
		}
		if i == 2 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected third sensitive request to be throttled, got %d", rec.Code)
		}
	}
}
