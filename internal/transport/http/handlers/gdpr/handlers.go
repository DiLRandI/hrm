package gdprhandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
	"hrm/internal/domain/gdpr"
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
	r.Route("/gdpr", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Get("/retention-policies", h.handleListRetention)
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Post("/retention-policies", h.handleCreateRetention)
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Get("/retention/runs", h.handleListRetentionRuns)
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Post("/retention/run", h.handleRunRetention)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Get("/dsar", h.handleListDSAR)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Post("/dsar", h.handleRequestDSAR)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Get("/dsar/{exportID}/download", h.handleDownloadDSAR)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Get("/anonymize", h.handleListAnonymizationJobs)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Post("/anonymize", h.handleRequestAnonymization)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Post("/anonymize/{jobID}/execute", h.handleExecuteAnonymization)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Get("/access-logs", h.handleAccessLogs)
	})
}

type retentionPolicy struct {
	ID            string `json:"id"`
	DataCategory  string `json:"dataCategory"`
	RetentionDays int    `json:"retentionDays"`
}

type retentionRun struct {
	ID           string    `json:"id"`
	DataCategory string    `json:"dataCategory"`
	CutoffDate   time.Time `json:"cutoffDate"`
	Status       string    `json:"status"`
	DeletedCount int64     `json:"deletedCount"`
	StartedAt    time.Time `json:"startedAt"`
	CompletedAt  time.Time `json:"completedAt"`
}

type anonymizationJob struct {
	ID          string    `json:"id"`
	EmployeeID  string    `json:"employeeId"`
	Status      string    `json:"status"`
	Reason      string    `json:"reason"`
	RequestedAt time.Time `json:"requestedAt"`
	CompletedAt any       `json:"completedAt"`
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

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "gdpr.retention.save", "retention_policy", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit gdpr.retention.save failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListRetentionRuns(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, data_category, cutoff_date, status, deleted_count, started_at, completed_at
    FROM retention_runs
    WHERE tenant_id = $1
    ORDER BY started_at DESC
    LIMIT 100
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "retention_runs_failed", "failed to list retention runs", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var runs []retentionRun
	for rows.Next() {
		var run retentionRun
		if err := rows.Scan(&run.ID, &run.DataCategory, &run.CutoffDate, &run.Status, &run.DeletedCount, &run.StartedAt, &run.CompletedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "retention_runs_failed", "failed to list retention runs", middleware.GetRequestID(r.Context()))
			return
		}
		runs = append(runs, run)
	}
	api.Success(w, runs, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleRunRetention(w http.ResponseWriter, r *http.Request) {
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
		DataCategory string `json:"dataCategory"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT data_category, retention_days
    FROM retention_policies
    WHERE tenant_id = $1
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "retention_run_failed", "failed to load retention policies", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	type runSummary struct {
		DataCategory string    `json:"dataCategory"`
		CutoffDate   time.Time `json:"cutoffDate"`
		Status       string    `json:"status"`
		DeletedCount int64     `json:"deletedCount"`
	}

	var summaries []runSummary
	for rows.Next() {
		var category string
		var retentionDays int
		if err := rows.Scan(&category, &retentionDays); err != nil {
			api.Fail(w, http.StatusInternalServerError, "retention_run_failed", "failed to read retention policies", middleware.GetRequestID(r.Context()))
			return
		}
		if payload.DataCategory != "" && payload.DataCategory != category {
			continue
		}
		if retentionDays <= 0 {
			continue
		}

		cutoff := time.Now().AddDate(0, 0, -retentionDays)
		deleted, err := h.applyRetention(r.Context(), user.TenantID, category, cutoff)
		status := "completed"
		if err != nil {
			status = "failed"
		}

		var runID string
		if err := h.DB.QueryRow(r.Context(), `
      INSERT INTO retention_runs (tenant_id, data_category, cutoff_date, status, deleted_count)
      VALUES ($1,$2,$3,$4,$5)
      RETURNING id
    `, user.TenantID, category, cutoff, status, deleted).Scan(&runID); err != nil {
			slog.Warn("retention run insert failed", "err", err)
		}

		summaries = append(summaries, runSummary{
			DataCategory: category,
			CutoffDate:   cutoff,
			Status:       status,
			DeletedCount: deleted,
		})
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "gdpr.retention.run", "retention_run", "", middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, summaries); err != nil {
		slog.Warn("audit gdpr.retention.run failed", "err", err)
	}
	api.Success(w, summaries, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListDSAR(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	query := `
    SELECT id, employee_id, requested_by, status, file_url, requested_at, completed_at
    FROM dsar_exports
    WHERE tenant_id = $1
  `
	args := []any{user.TenantID}
	if user.RoleName != auth.RoleHR {
		var employeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			slog.Warn("dsar list employee lookup failed", "err", err)
		}
		if employeeID == "" {
			api.Success(w, []map[string]any{}, middleware.GetRequestID(r.Context()))
			return
		}
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	query += " ORDER BY requested_at DESC"

	rows, err := h.DB.Query(r.Context(), query, args...)
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
	if payload.EmployeeID == "" {
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&payload.EmployeeID); err != nil {
			slog.Warn("dsar request employee lookup failed", "err", err)
		}
	}
	if payload.EmployeeID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee id required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		var selfEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
			slog.Warn("dsar request self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || payload.EmployeeID != selfEmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
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
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "gdpr.dsar.request", "dsar_export", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit gdpr.dsar.request failed", "err", err)
	}

	status := gdpr.DSARStatusProcessing
	fileURL, err := h.generateDSAR(r.Context(), user.TenantID, payload.EmployeeID, id)
	if err == nil {
		status = gdpr.DSARStatusCompleted
		if _, err := h.DB.Exec(r.Context(), `
      UPDATE dsar_exports SET status = $1, file_url = $2, completed_at = now() WHERE id = $3
    `, gdpr.DSARStatusCompleted, fileURL, id); err != nil {
			slog.Warn("dsar complete update failed", "err", err)
		}
		if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "gdpr.dsar.complete", "dsar_export", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"fileUrl": fileURL}); err != nil {
			slog.Warn("audit gdpr.dsar.complete failed", "err", err)
		}
	} else {
		status = gdpr.DSARStatusFailed
		if _, err := h.DB.Exec(r.Context(), `
      UPDATE dsar_exports SET status = $1 WHERE id = $2
    `, gdpr.DSARStatusFailed, id); err != nil {
			slog.Warn("dsar failed update failed", "err", err)
		}
	}

	api.Created(w, map[string]string{"id": id, "status": status}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleDownloadDSAR(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	exportID := chi.URLParam(r, "exportID")
	var employeeID, fileURL string
	err := h.DB.QueryRow(r.Context(), `
    SELECT employee_id, COALESCE(file_url, '')
    FROM dsar_exports
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, exportID).Scan(&employeeID, &fileURL)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "dsar export not found", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName != auth.RoleHR {
		var selfEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
			slog.Warn("dsar download self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || selfEmployeeID != employeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if fileURL == "" {
		api.Fail(w, http.StatusNotFound, "not_found", "dsar file not available", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := os.Stat(fileURL); err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "dsar file not found", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=dsar-%s.json", exportID))
	http.ServeFile(w, r, fileURL)
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "gdpr.dsar.download", "dsar_export", exportID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		slog.Warn("audit gdpr.dsar.download failed", "err", err)
	}
}

func (h *Handler) generateDSAR(ctx context.Context, tenantID, employeeID, exportID string) (string, error) {
	var employeeJSON []byte
	if err := h.DB.QueryRow(ctx, `
    SELECT row_to_json(e)
    FROM employees e
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, employeeID).Scan(&employeeJSON); err != nil {
		return "", err
	}
	var employee map[string]any
	if err := json.Unmarshal(employeeJSON, &employee); err != nil {
		return "", err
	}

	var leaveRequests []map[string]any
	rows, err := h.DB.Query(ctx, `
    SELECT row_to_json(lr)
    FROM leave_requests lr
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var rowJSON []byte
		if err := rows.Scan(&rowJSON); err != nil {
			rows.Close()
			return "", err
		}
		var row map[string]any
		if err := json.Unmarshal(rowJSON, &row); err != nil {
			rows.Close()
			return "", err
		}
		leaveRequests = append(leaveRequests, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	rows.Close()

	var payrollResults []map[string]any
	rows, err = h.DB.Query(ctx, `
    SELECT row_to_json(pr)
    FROM payroll_results pr
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var rowJSON []byte
		if err := rows.Scan(&rowJSON); err != nil {
			rows.Close()
			return "", err
		}
		var row map[string]any
		if err := json.Unmarshal(rowJSON, &row); err != nil {
			rows.Close()
			return "", err
		}
		payrollResults = append(payrollResults, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	rows.Close()

	var goals []map[string]any
	rows, err = h.DB.Query(ctx, `
    SELECT row_to_json(g)
    FROM goals g
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var rowJSON []byte
		if err := rows.Scan(&rowJSON); err != nil {
			rows.Close()
			return "", err
		}
		var row map[string]any
		if err := json.Unmarshal(rowJSON, &row); err != nil {
			rows.Close()
			return "", err
		}
		goals = append(goals, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
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

func (h *Handler) handleListAnonymizationJobs(w http.ResponseWriter, r *http.Request) {
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
    SELECT id, employee_id, status, reason, requested_at, completed_at
    FROM anonymization_jobs
    WHERE tenant_id = $1
    ORDER BY requested_at DESC
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "anonymize_list_failed", "failed to list anonymization jobs", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var jobs []anonymizationJob
	for rows.Next() {
		var job anonymizationJob
		if err := rows.Scan(&job.ID, &job.EmployeeID, &job.Status, &job.Reason, &job.RequestedAt, &job.CompletedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "anonymize_list_failed", "failed to list anonymization jobs", middleware.GetRequestID(r.Context()))
			return
		}
		jobs = append(jobs, job)
	}
	api.Success(w, jobs, middleware.GetRequestID(r.Context()))
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

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "gdpr.anonymize.request", "anonymization_job", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit gdpr.anonymize.request failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id, "status": gdpr.AnonymizationRequested}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleExecuteAnonymization(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	jobID := chi.URLParam(r, "jobID")
	ctx := r.Context()

	var employeeID, status string
	err := h.DB.QueryRow(ctx, `
    SELECT employee_id, status
    FROM anonymization_jobs
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, jobID).Scan(&employeeID, &status)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "anonymization job not found", middleware.GetRequestID(r.Context()))
		return
	}
	if status != gdpr.AnonymizationRequested {
		api.Fail(w, http.StatusBadRequest, "invalid_state", "anonymization job is not in requested state", middleware.GetRequestID(r.Context()))
		return
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to start anonymization", middleware.GetRequestID(r.Context()))
		return
	}
	defer tx.Rollback(ctx)

	execTx := func(query string, args ...any) error {
		_, execErr := tx.Exec(ctx, query, args...)
		return execErr
	}

	if err := execTx(`
    UPDATE anonymization_jobs
    SET status = $1
    WHERE tenant_id = $2 AND id = $3
  `, gdpr.AnonymizationProcessing, user.TenantID, jobID); err != nil {
		h.failAnonymization(ctx, user.TenantID, jobID)
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to start anonymization", middleware.GetRequestID(r.Context()))
		return
	}

	var userID string
	if err := tx.QueryRow(ctx, `
    SELECT user_id
    FROM employees
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, employeeID).Scan(&userID); err != nil {
		slog.Warn("anonymization user lookup failed", "err", err)
	}

	anonEmployeeEmail := fmt.Sprintf("anonymized+%s@example.local", employeeID)
	if err := execTx(`
    UPDATE employees
    SET employee_number = NULL,
        first_name = 'Anonymized',
        last_name = 'Employee',
        email = $1,
        phone = NULL,
        address = NULL,
        national_id = NULL,
        bank_account = NULL,
        salary = NULL,
        currency = COALESCE(currency, 'USD'),
        employment_type = NULL,
        department_id = NULL,
        manager_id = NULL,
        status = $2,
        updated_at = now()
    WHERE tenant_id = $3 AND id = $4
  `, anonEmployeeEmail, core.EmployeeStatusAnonymized, user.TenantID, employeeID); err != nil {
		h.failAnonymization(ctx, user.TenantID, jobID)
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to anonymize employee", middleware.GetRequestID(r.Context()))
		return
	}

	if userID != "" {
		anonUserEmail := fmt.Sprintf("anonymized+%s@example.local", userID)
		if err := execTx(`
      UPDATE users
      SET email = $1,
          status = $2,
          updated_at = now()
      WHERE tenant_id = $3 AND id = $4
    `, anonUserEmail, core.UserStatusDisabled, user.TenantID, userID); err != nil {
			h.failAnonymization(ctx, user.TenantID, jobID)
			api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to anonymize user", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if err := execTx(`
    UPDATE leave_requests
    SET reason = NULL
    WHERE tenant_id = $1 AND employee_id = $2
  `, user.TenantID, employeeID); err != nil {
		h.failAnonymization(ctx, user.TenantID, jobID)
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to anonymize leave requests", middleware.GetRequestID(r.Context()))
		return
	}

	if err := execTx(`
    UPDATE goals
    SET title = 'Anonymized goal', description = NULL, metric = NULL, updated_at = now()
    WHERE tenant_id = $1 AND employee_id = $2
  `, user.TenantID, employeeID); err != nil {
		h.failAnonymization(ctx, user.TenantID, jobID)
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to anonymize goals", middleware.GetRequestID(r.Context()))
		return
	}

	if err := execTx(`
    UPDATE feedback
    SET message = 'Anonymized'
    WHERE tenant_id = $1 AND to_employee_id = $2
  `, user.TenantID, employeeID); err != nil {
		h.failAnonymization(ctx, user.TenantID, jobID)
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to anonymize feedback", middleware.GetRequestID(r.Context()))
		return
	}

	if err := execTx(`
    UPDATE checkins
    SET notes = 'Anonymized', private = true
    WHERE tenant_id = $1 AND employee_id = $2
  `, user.TenantID, employeeID); err != nil {
		h.failAnonymization(ctx, user.TenantID, jobID)
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to anonymize checkins", middleware.GetRequestID(r.Context()))
		return
	}

	if err := execTx(`
    UPDATE pips
    SET objectives_json = NULL, milestones_json = NULL, review_dates_json = NULL, updated_at = now()
    WHERE tenant_id = $1 AND employee_id = $2
  `, user.TenantID, employeeID); err != nil {
		h.failAnonymization(ctx, user.TenantID, jobID)
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to anonymize pips", middleware.GetRequestID(r.Context()))
		return
	}

	if err := execTx(`
    UPDATE payslips
    SET file_url = NULL
    WHERE tenant_id = $1 AND employee_id = $2
  `, user.TenantID, employeeID); err != nil {
		h.failAnonymization(ctx, user.TenantID, jobID)
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to anonymize payslips", middleware.GetRequestID(r.Context()))
		return
	}

	if err := execTx(`
    UPDATE anonymization_jobs
    SET status = $1, completed_at = now()
    WHERE tenant_id = $2 AND id = $3
  `, gdpr.AnonymizationCompleted, user.TenantID, jobID); err != nil {
		h.failAnonymization(ctx, user.TenantID, jobID)
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to complete anonymization", middleware.GetRequestID(r.Context()))
		return
	}

	if err := tx.Commit(ctx); err != nil {
		h.failAnonymization(ctx, user.TenantID, jobID)
		api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to commit anonymization", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(ctx, user.TenantID, user.UserID, "gdpr.anonymize.execute", "anonymization_job", jobID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"employeeId": employeeID}); err != nil {
		slog.Warn("audit gdpr.anonymize.execute failed", "err", err)
	}
	api.Success(w, map[string]string{"status": gdpr.AnonymizationCompleted}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) failAnonymization(ctx context.Context, tenantID, jobID string) {
	if _, err := h.DB.Exec(ctx, `
    UPDATE anonymization_jobs
    SET status = $1
    WHERE tenant_id = $2 AND id = $3
  `, gdpr.AnonymizationFailed, tenantID, jobID); err != nil {
		slog.Warn("anonymization fail update failed", "err", err)
	}
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

func (h *Handler) applyRetention(ctx context.Context, tenantID, category string, cutoff time.Time) (int64, error) {
	switch category {
	case gdpr.DataCategoryAudit:
		tag, err := h.DB.Exec(ctx, `
      DELETE FROM audit_events
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		return tag.RowsAffected(), err
	case gdpr.DataCategoryLeave:
		var total int64
		tag, err := h.DB.Exec(ctx, `
      DELETE FROM leave_approvals
      WHERE tenant_id = $1 AND decided_at IS NOT NULL AND decided_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = h.DB.Exec(ctx, `
      DELETE FROM leave_requests
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		return total, err
	case gdpr.DataCategoryPayroll:
		rows, err := h.DB.Query(ctx, `
      SELECT id
      FROM payroll_periods
      WHERE tenant_id = $1 AND end_date < $2
    `, tenantID, cutoff)
		if err != nil {
			return 0, err
		}
		var periodIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return 0, err
			}
			periodIDs = append(periodIDs, id)
		}
		rows.Close()
		if len(periodIDs) == 0 {
			return 0, nil
		}
		var total int64
		queries := []string{
			"DELETE FROM payroll_inputs WHERE tenant_id = $1 AND period_id = ANY($2::uuid[])",
			"DELETE FROM payroll_adjustments WHERE tenant_id = $1 AND period_id = ANY($2::uuid[])",
			"DELETE FROM payroll_results WHERE tenant_id = $1 AND period_id = ANY($2::uuid[])",
			"DELETE FROM payslips WHERE tenant_id = $1 AND period_id = ANY($2::uuid[])",
			"DELETE FROM journal_exports WHERE tenant_id = $1 AND period_id = ANY($2::uuid[])",
			"DELETE FROM payroll_periods WHERE tenant_id = $1 AND id = ANY($2::uuid[])",
		}
		for _, q := range queries {
			tag, err := h.DB.Exec(ctx, q, tenantID, periodIDs)
			total += tag.RowsAffected()
			if err != nil {
				return total, err
			}
		}
		return total, nil
	case gdpr.DataCategoryPerformance:
		var total int64
		tag, err := h.DB.Exec(ctx, `
      UPDATE feedback
      SET related_goal_id = NULL
      WHERE tenant_id = $1
        AND related_goal_id IN (SELECT id FROM goals WHERE tenant_id = $1 AND updated_at < $2)
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}

		tag, err = h.DB.Exec(ctx, `
      DELETE FROM review_responses
      WHERE tenant_id = $1 AND submitted_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = h.DB.Exec(ctx, `
      DELETE FROM review_tasks
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = h.DB.Exec(ctx, `
      DELETE FROM review_cycles
      WHERE tenant_id = $1 AND end_date < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = h.DB.Exec(ctx, `
      DELETE FROM goal_comments
      WHERE goal_id IN (SELECT id FROM goals WHERE tenant_id = $1 AND updated_at < $2)
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = h.DB.Exec(ctx, `
      DELETE FROM goals
      WHERE tenant_id = $1 AND updated_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = h.DB.Exec(ctx, `
      DELETE FROM feedback
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = h.DB.Exec(ctx, `
      DELETE FROM checkins
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = h.DB.Exec(ctx, `
      DELETE FROM pips
      WHERE tenant_id = $1 AND updated_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		return total, err
	case gdpr.DataCategoryGDPR:
		var total int64
		tag, err := h.DB.Exec(ctx, `
      DELETE FROM dsar_exports
      WHERE tenant_id = $1 AND completed_at IS NOT NULL AND completed_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = h.DB.Exec(ctx, `
      DELETE FROM anonymization_jobs
      WHERE tenant_id = $1 AND completed_at IS NOT NULL AND completed_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		return total, err
	case gdpr.DataCategoryAccessLogs:
		tag, err := h.DB.Exec(ctx, `
      DELETE FROM access_logs
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		return tag.RowsAffected(), err
	case gdpr.DataCategoryNotifications:
		tag, err := h.DB.Exec(ctx, `
      DELETE FROM notifications
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		return tag.RowsAffected(), err
	default:
		return 0, nil
	}
}
