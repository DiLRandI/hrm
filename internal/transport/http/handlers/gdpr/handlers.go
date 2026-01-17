package gdprhandler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/gdpr"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
)

type Handler struct {
	DB *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{DB: db}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/gdpr", func(r chi.Router) {
		r.Get("/retention-policies", h.handleListRetention)
		r.Post("/retention-policies", h.handleCreateRetention)
		r.Get("/dsar", h.handleListDSAR)
		r.Post("/dsar", h.handleRequestDSAR)
		r.Post("/anonymize", h.handleRequestAnonymization)
		r.Get("/access-logs", h.handleAccessLogs)
	})
}

type retentionPolicy struct {
	ID            string `json:"id"`
	DataCategory  string `json:"dataCategory"`
	RetentionDays int    `json:"retentionDays"`
}

func (h *Handler) handleListRetention(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, data_category, retention_days
    FROM retention_policies
    WHERE tenant_id = $1
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "retention_list_failed", "failed to list retention policies", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var policies []retentionPolicy
	for rows.Next() {
		var p retentionPolicy
		if err := rows.Scan(&p.ID, &p.DataCategory, &p.RetentionDays); err != nil {
			api.Fail(w, http.StatusInternalServerError, "retention_list_failed", "failed to list retention policies", middleware.GetRequestID(r.Context()))
			return
		}
		policies = append(policies, p)
	}
	api.Success(w, policies, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateRetention(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload retentionPolicy
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO retention_policies (tenant_id, data_category, retention_days)
    VALUES ($1,$2,$3)
    ON CONFLICT (tenant_id, data_category) DO UPDATE SET retention_days = EXCLUDED.retention_days
    RETURNING id
  `, user.TenantID, payload.DataCategory, payload.RetentionDays).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "retention_create_failed", "failed to save retention policy", middleware.GetRequestID(r.Context()))
		return
	}

	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListDSAR(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, employee_id, requested_by, status, file_url, requested_at, completed_at
    FROM dsar_exports
    WHERE tenant_id = $1
    ORDER BY requested_at DESC
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "dsar_list_failed", "failed to list dsar exports", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var exports []map[string]any
	for rows.Next() {
		var id, employeeID, requestedBy, status, fileURL string
		var requestedAt time.Time
		var completedAt any
		if err := rows.Scan(&id, &employeeID, &requestedBy, &status, &fileURL, &requestedAt, &completedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "dsar_list_failed", "failed to list dsar exports", middleware.GetRequestID(r.Context()))
			return
		}
		exports = append(exports, map[string]any{
			"id":          id,
			"employeeId":  employeeID,
			"requestedBy": requestedBy,
			"status":      status,
			"fileUrl":     fileURL,
			"requestedAt": requestedAt,
			"completedAt": completedAt,
		})
	}
	api.Success(w, exports, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleRequestDSAR(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload struct {
		EmployeeID string `json:"employeeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO dsar_exports (tenant_id, employee_id, requested_by, status)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, user.TenantID, payload.EmployeeID, user.UserID, gdpr.DSARStatusProcessing).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "dsar_request_failed", "failed to request dsar", middleware.GetRequestID(r.Context()))
		return
	}

	fileURL, err := h.generateDSAR(r.Context(), user.TenantID, payload.EmployeeID, id)
	if err == nil {
		_, _ = h.DB.Exec(r.Context(), `
      UPDATE dsar_exports SET status = $1, file_url = $2, completed_at = now() WHERE id = $3
    `, gdpr.DSARStatusCompleted, fileURL, id)
	}

	api.Created(w, map[string]string{"id": id, "status": gdpr.DSARStatusProcessing}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) generateDSAR(ctx context.Context, tenantID, employeeID, exportID string) (string, error) {
	var employeeJSON []byte
	_ = h.DB.QueryRow(ctx, `
    SELECT row_to_json(e)
    FROM employees e
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, employeeID).Scan(&employeeJSON)
	var employee map[string]any
	_ = json.Unmarshal(employeeJSON, &employee)

	var leaveRequests []map[string]any
	rows, _ := h.DB.Query(ctx, `
    SELECT row_to_json(lr)
    FROM leave_requests lr
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	for rows.Next() {
		var rowJSON []byte
		_ = rows.Scan(&rowJSON)
		var row map[string]any
		_ = json.Unmarshal(rowJSON, &row)
		leaveRequests = append(leaveRequests, row)
	}
	rows.Close()

	var payrollResults []map[string]any
	rows, _ = h.DB.Query(ctx, `
    SELECT row_to_json(pr)
    FROM payroll_results pr
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	for rows.Next() {
		var rowJSON []byte
		_ = rows.Scan(&rowJSON)
		var row map[string]any
		_ = json.Unmarshal(rowJSON, &row)
		payrollResults = append(payrollResults, row)
	}
	rows.Close()

	var goals []map[string]any
	rows, _ = h.DB.Query(ctx, `
    SELECT row_to_json(g)
    FROM goals g
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	for rows.Next() {
		var rowJSON []byte
		_ = rows.Scan(&rowJSON)
		var row map[string]any
		_ = json.Unmarshal(rowJSON, &row)
		goals = append(goals, row)
	}
	rows.Close()

	payload := gdpr.BuildDSARPayload(employee, leaveRequests, payrollResults, goals)
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll("storage/dsar", 0o755); err != nil {
		return "", err
	}

	filePath := filepath.Join("storage/dsar", exportID+".json")
	if err := os.WriteFile(filePath, jsonBytes, 0o600); err != nil {
		return "", err
	}

	return filePath, nil
}

func (h *Handler) handleRequestAnonymization(w http.ResponseWriter, r *http.Request) {
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
		EmployeeID string `json:"employeeId"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO anonymization_jobs (tenant_id, employee_id, status, reason)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, user.TenantID, payload.EmployeeID, gdpr.AnonymizationRequested, payload.Reason).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "anonymize_request_failed", "failed to request anonymization", middleware.GetRequestID(r.Context()))
		return
	}

	api.Created(w, map[string]string{"id": id, "status": gdpr.AnonymizationRequested}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleAccessLogs(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, actor_user_id, employee_id, fields, request_id, created_at
    FROM access_logs
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "access_log_failed", "failed to list access logs", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var logs []map[string]any
	for rows.Next() {
		var id, actorID, employeeID, requestID string
		var fields []string
		var createdAt time.Time
		if err := rows.Scan(&id, &actorID, &employeeID, &fields, &requestID, &createdAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "access_log_failed", "failed to list access logs", middleware.GetRequestID(r.Context()))
			return
		}
		logs = append(logs, map[string]any{
			"id":          id,
			"actorUserId": actorID,
			"employeeId":  employeeID,
			"fields":      fields,
			"requestId":   requestID,
			"createdAt":   createdAt,
		})
	}

	api.Success(w, logs, middleware.GetRequestID(r.Context()))
}
