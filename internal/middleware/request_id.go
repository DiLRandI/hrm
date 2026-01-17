package middleware

import (
  "context"
  "net/http"

  "github.com/google/uuid"
)

func RequestID(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    reqID := r.Header.Get("X-Request-ID")
    if reqID == "" {
      reqID = uuid.NewString()
    }
    w.Header().Set("X-Request-ID", reqID)
    ctx := context.WithValue(r.Context(), ctxKeyRequestID, reqID)
    next.ServeHTTP(w, r.WithContext(ctx))
  })
}

func GetRequestID(ctx context.Context) string {
  if value, ok := ctx.Value(ctxKeyRequestID).(string); ok {
    return value
  }
  return ""
}
