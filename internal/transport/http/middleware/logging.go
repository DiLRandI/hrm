package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"hrm/internal/platform/metrics"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func Logger(collector *metrics.Collector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)
			duration := time.Since(start)
			if collector != nil {
				collector.Record(recorder.status, duration)
			}
			slog.Info("http.request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"durationMs", duration.Milliseconds(),
				"requestId", GetRequestID(r.Context()),
			)
		})
	}
}
