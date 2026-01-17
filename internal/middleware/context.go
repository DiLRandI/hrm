package middleware

type ctxKey string

const (
  ctxKeyRequestID ctxKey = "request_id"
  ctxKeyUser      ctxKey = "user"
)
