package notificationshandler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/notifications"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	Service *notifications.Service
}

func NewHandler(service *notifications.Service) *Handler {
	return &Handler{Service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/notifications", func(r chi.Router) {
		r.Get("/", h.handleList)
		r.Post("/{notificationID}/read", h.handleMarkRead)
		r.Get("/settings", h.handleSettings)
		r.Put("/settings", h.handleUpdateSettings)
	})
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	page := shared.ParsePagination(r, 100, 500)
	total, err := h.Service.Count(r.Context(), user.TenantID, user.UserID)
	if err != nil {
		slog.Warn("notification count failed", "err", err)
	}

	items, err := h.Service.List(r.Context(), user.TenantID, user.UserID, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "notification_list_failed", "failed to list notifications", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	api.Success(w, items, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	notificationID := chi.URLParam(r, "notificationID")
	if err := h.Service.MarkRead(r.Context(), user.TenantID, user.UserID, notificationID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "notification_update_failed", "failed to update notification", middleware.GetRequestID(r.Context()))
		return
	}

	api.Success(w, map[string]string{"status": "read"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	enabled, from, err := h.Service.GetSettings(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "settings_failed", "failed to load settings", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, map[string]any{"emailEnabled": enabled, "emailFrom": from}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload struct {
		EmailEnabled bool   `json:"emailEnabled"`
		EmailFrom    string `json:"emailFrom"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.UpdateSettings(r.Context(), user.TenantID, payload.EmailEnabled, payload.EmailFrom); err != nil {
		api.Fail(w, http.StatusInternalServerError, "settings_failed", "failed to update settings", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, map[string]string{"status": "updated"}, middleware.GetRequestID(r.Context()))
}
