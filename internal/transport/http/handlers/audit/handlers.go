package audithandler

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/auth"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	DB    *pgxpool.Pool
	Perms middleware.PermissionStore
}

func NewHandler(db *pgxpool.Pool, perms middleware.PermissionStore) *Handler {
	return &Handler{DB: db, Perms: perms}
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

	query := `
    SELECT id, actor_user_id, action, entity_type, entity_id, request_id, ip, created_at`
	if includeDetails {
		query += ", before_json, after_json"
	}
	query += `
    FROM audit_events
    WHERE tenant_id = $1`
	args := []any{user.TenantID}
	if action != "" {
		query += fmt.Sprintf(" AND action = $%d", len(args)+1)
		args = append(args, action)
	}
	if entity != "" {
		query += fmt.Sprintf(" AND entity_type = $%d", len(args)+1)
		args = append(args, entity)
	}
	if actor != "" {
		query += fmt.Sprintf(" AND actor_user_id::text = $%d", len(args)+1)
		args = append(args, actor)
	}

	countQuery := "SELECT COUNT(1) FROM audit_events WHERE tenant_id = $1"
	if action != "" {
		countQuery += " AND action = $2"
		if entity != "" {
			countQuery += " AND entity_type = $3"
			if actor != "" {
				countQuery += " AND actor_user_id::text = $4"
			}
		} else if actor != "" {
			countQuery += " AND actor_user_id::text = $3"
		}
	} else if entity != "" {
		countQuery += " AND entity_type = $2"
		if actor != "" {
			countQuery += " AND actor_user_id::text = $3"
		}
	} else if actor != "" {
		countQuery += " AND actor_user_id::text = $2"
	}
	var total int
	if err := h.DB.QueryRow(r.Context(), countQuery, args...).Scan(&total); err != nil {
		slog.Warn("audit count failed", "err", err)
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", limitPos, offsetPos)
	args = append(args, page.Limit, page.Offset)

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "audit_list_failed", "failed to list audit events", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, actorID, actionVal, entityType, entityID, requestID, ip string
		var createdAt any
		if includeDetails {
			var beforeJSON, afterJSON any
			if err := rows.Scan(&id, &actorID, &actionVal, &entityType, &entityID, &requestID, &ip, &createdAt, &beforeJSON, &afterJSON); err != nil {
				api.Fail(w, http.StatusInternalServerError, "audit_list_failed", "failed to list audit events", middleware.GetRequestID(r.Context()))
				return
			}
			out = append(out, map[string]any{
				"id":         id,
				"actorId":    actorID,
				"action":     actionVal,
				"entityType": entityType,
				"entityId":   entityID,
				"requestId":  requestID,
				"ip":         ip,
				"createdAt":  createdAt,
				"before":     beforeJSON,
				"after":      afterJSON,
			})
		} else {
			if err := rows.Scan(&id, &actorID, &actionVal, &entityType, &entityID, &requestID, &ip, &createdAt); err != nil {
				api.Fail(w, http.StatusInternalServerError, "audit_list_failed", "failed to list audit events", middleware.GetRequestID(r.Context()))
				return
			}
			out = append(out, map[string]any{
				"id":         id,
				"actorId":    actorID,
				"action":     actionVal,
				"entityType": entityType,
				"entityId":   entityID,
				"requestId":  requestID,
				"ip":         ip,
				"createdAt":  createdAt,
			})
		}
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	api.Success(w, out, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleExportEvents(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, actor_user_id, action, entity_type, entity_id, request_id, ip, created_at
    FROM audit_events
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "audit_export_failed", "failed to export audit events", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=audit-events.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"id", "actor_user_id", "action", "entity_type", "entity_id", "request_id", "ip", "created_at"}); err != nil {
		slog.Warn("audit export header failed", "err", err)
	}
	for rows.Next() {
		var id, actorID, actionVal, entityType, entityID, requestID, ip string
		var createdAt any
		if err := rows.Scan(&id, &actorID, &actionVal, &entityType, &entityID, &requestID, &ip, &createdAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "audit_export_failed", "failed to export audit events", middleware.GetRequestID(r.Context()))
			return
		}
		if err := writer.Write([]string{id, actorID, actionVal, entityType, entityID, requestID, ip, fmt.Sprint(createdAt)}); err != nil {
			slog.Warn("audit export row failed", "err", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		slog.Warn("audit export flush failed", "err", err)
	}
}
