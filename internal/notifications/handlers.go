package notifications

import (
  "net/http"

  "github.com/go-chi/chi/v5"

  "hrm/internal/api"
  "hrm/internal/middleware"
)

type Handler struct {
  Service *Service
}

func NewHandler(service *Service) *Handler {
  return &Handler{Service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
  r.Route("/notifications", func(r chi.Router) {
    r.Get("/", h.handleList)
    r.Post("/{notificationID}/read", h.handleMarkRead)
  })
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
  user, ok := middleware.GetUser(r.Context())
  if !ok {
    api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
    return
  }

  items, err := h.Service.List(r.Context(), user.TenantID, user.UserID)
  if err != nil {
    api.Fail(w, http.StatusInternalServerError, "notification_list_failed", "failed to list notifications", middleware.GetRequestID(r.Context()))
    return
  }

  api.Success(w, items, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleMarkRead(w http.ResponseWriter, r *http.Request) {
  user, ok := middleware.GetUser(r.Context())
  if !ok {
    api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
    return
  }

  notificationID := chi.URLParam(r, "notificationID")
  _, err := h.Service.DB.Exec(r.Context(), `
    UPDATE notifications SET read_at = now()
    WHERE tenant_id = $1 AND user_id = $2 AND id = $3
  `, user.TenantID, user.UserID, notificationID)
  if err != nil {
    api.Fail(w, http.StatusInternalServerError, "notification_update_failed", "failed to update notification", middleware.GetRequestID(r.Context()))
    return
  }

  api.Success(w, map[string]string{"status": "read"}, middleware.GetRequestID(r.Context()))
}
