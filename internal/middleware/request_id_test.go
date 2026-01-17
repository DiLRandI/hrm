package middleware

import (
  "net/http"
  "net/http/httptest"
  "testing"
)

func TestRequestIDMiddleware(t *testing.T) {
  handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    reqID := GetRequestID(r.Context())
    if reqID == "" {
      t.Fatal("expected request id in context")
    }
  }))

  req := httptest.NewRequest(http.MethodGet, "/", nil)
  rec := httptest.NewRecorder()
  handler.ServeHTTP(rec, req)

  if rec.Header().Get("X-Request-ID") == "" {
    t.Fatal("expected request id header")
  }
}
