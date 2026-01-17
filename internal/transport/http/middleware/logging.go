package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type logEntry struct {
	Timestamp string `json:"ts"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	Duration  int64  `json:"durationMs"`
	RequestID string `json:"requestId"`
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		entry := logEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Method:    r.Method,
			Path:      r.URL.Path,
			Status:    recorder.status,
			Duration:  time.Since(start).Milliseconds(),
			RequestID: GetRequestID(r.Context()),
		}

		payload, _ := json.Marshal(entry)
		log.Println(string(payload))
	})
}
