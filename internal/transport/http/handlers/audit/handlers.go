package audithandler

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	Service *audit.Service
	Perms   middleware.PermissionStore
}

func NewHandler(service *audit.Service, perms middleware.PermissionStore) *Handler {
	return &Handler{Service: service, Perms: perms}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/audit", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermAuditRead, h.Perms)).Get("/events", h.handleListEvents)
		r.With(middleware.RequirePermission(auth.PermAuditRead, h.Perms)).Get("/events/export", h.handleExportEvents)
	})
}

func (h *Handler) handleListEvents(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	page := shared.ParsePagination(r, 100, 500)
	action := r.URL.Query().Get("action")
	entity := r.URL.Query().Get("entityType")
	actor := r.URL.Query().Get("actorUserId")
	includeDetails := r.URL.Query().Get("includeDetails") == "true"
	filter := audit.Filter{Action: action, EntityType: entity, ActorUser: actor}
	total, err := h.Service.Count(r.Context(), user.TenantID, filter)
	if err != nil {
		slog.Warn("audit count failed", "err", err)
	}

	events, err := h.Service.List(r.Context(), user.TenantID, filter, includeDetails, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "audit_list_failed", "failed to list audit events", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	api.Success(w, events, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleExportEvents(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	events, err := h.Service.ListExport(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "audit_export_failed", "failed to export audit events", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=audit-events.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"id", "actor_user_id", "action", "entity_type", "entity_id", "request_id", "ip", "created_at"}); err != nil {
		slog.Warn("audit export header failed", "err", err)
	}
	for _, evt := range events {
		if err := writer.Write([]string{evt.ID, evt.ActorID, evt.Action, evt.EntityType, evt.EntityID, evt.RequestID, evt.IP, fmt.Sprint(evt.CreatedAt)}); err != nil {
			slog.Warn("audit export row failed", "err", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		slog.Warn("audit export flush failed", "err", err)
	}
}
