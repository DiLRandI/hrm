package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Envelope struct {
	Success   bool   `json:"success"`
	Data      any    `json:"data,omitempty"`
	Error     *Error `json:"error,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, payload Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Warn("write json failed", "err", err)
	}
}

func Success(w http.ResponseWriter, data any, requestID string) {
	WriteJSON(w, http.StatusOK, Envelope{Success: true, Data: data, RequestID: requestID})
}

func Created(w http.ResponseWriter, data any, requestID string) {
	WriteJSON(w, http.StatusCreated, Envelope{Success: true, Data: data, RequestID: requestID})
}

func Fail(w http.ResponseWriter, status int, code, message, requestID string) {
	WriteJSON(w, status, Envelope{Success: false, Error: &Error{Code: code, Message: message}, RequestID: requestID})
}
