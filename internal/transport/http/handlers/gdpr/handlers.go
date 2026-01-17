package gdprhandler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
	"hrm/internal/domain/gdpr"
	cryptoutil "hrm/internal/platform/crypto"
	"hrm/internal/platform/jobs"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	DB     *pgxpool.Pool
	Perms  middleware.PermissionStore
	Crypto *cryptoutil.Service
	Jobs   *jobs.Service
	Store  *core.Store
}

func NewHandler(db *pgxpool.Pool, store *core.Store, crypto *cryptoutil.Service, jobsSvc *jobs.Service) *Handler {
	return &Handler{DB: db, Perms: store, Store: store, Crypto: crypto, Jobs: jobsSvc}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/gdpr", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Get("/retention-policies", h.handleListRetention)
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Post("/retention-policies", h.handleCreateRetention)
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Get("/retention/runs", h.handleListRetentionRuns)
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Post("/retention/run", h.handleRunRetention)
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Get("/consents", h.handleListConsents)
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Post("/consents", h.handleCreateConsent)
		r.With(middleware.RequirePermission(auth.PermGDPRRetention, h.Perms)).Post("/consents/{consentID}/revoke", h.handleRevokeConsent)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Get("/dsar", h.handleListDSAR)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Post("/dsar", h.handleRequestDSAR)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Get("/dsar/{exportID}/download", h.handleDownloadDSAR)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Get("/anonymize", h.handleListAnonymizationJobs)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Post("/anonymize", h.handleRequestAnonymization)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Post("/anonymize/{jobID}/execute", h.handleExecuteAnonymization)
		r.With(middleware.RequirePermission(auth.PermGDPRExport, h.Perms)).Get("/anonymize/{jobID}/download", h.handleDownloadAnonymizationReport)
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
	ID            string    `json:"id"`
	EmployeeID    string    `json:"employeeId"`
	Status        string    `json:"status"`
	Reason        string    `json:"reason"`
	RequestedAt   time.Time `json:"requestedAt"`
	CompletedAt   any       `json:"completedAt"`
	FilePath      string    `json:"filePath,omitempty"`
	DownloadToken string    `json:"downloadToken,omitempty"`
}

type consentRecord struct {
	ID          string `json:"id"`
	EmployeeID  string `json:"employeeId"`
	ConsentType string `json:"consentType"`
	GrantedAt   any    `json:"grantedAt"`
	RevokedAt   any    `json:"revokedAt"`
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
		runID := ""
		if h.Jobs != nil {
			if err := h.DB.QueryRow(r.Context(), `
        INSERT INTO job_runs (tenant_id, job_type, status)
        VALUES ($1,$2,$3)
        RETURNING id
      `, user.TenantID, jobs.JobRetention, "running").Scan(&runID); err != nil {
				slog.Warn("retention job run insert failed", "err", err)
			}
		}

		deleted, err := gdpr.ApplyRetention(r.Context(), h.DB, user.TenantID, category, cutoff)
		status := "completed"
		if err != nil {
			status = "failed"
		}

		if runID != "" {
			detailsJSON, err := json.Marshal(map[string]any{
				"dataCategory": category,
				"cutoffDate":   cutoff,
				"deleted":      deleted,
			})
			if err != nil {
				slog.Warn("retention job details marshal failed", "err", err)
				detailsJSON = []byte("{}")
			}
			if _, updErr := h.DB.Exec(r.Context(), `
        UPDATE job_runs SET status = $1, details_json = $2, completed_at = now()
        WHERE id = $3
      `, status, detailsJSON, runID); updErr != nil {
				slog.Warn("retention job run update failed", "err", updErr)
			}
		}

		var retentionRunID string
		if err := h.DB.QueryRow(r.Context(), `
      INSERT INTO retention_runs (tenant_id, data_category, cutoff_date, status, deleted_count)
      VALUES ($1,$2,$3,$4,$5)
      RETURNING id
    `, user.TenantID, category, cutoff, status, deleted).Scan(&retentionRunID); err != nil {
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

func (h *Handler) handleListConsents(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	employeeID := r.URL.Query().Get("employeeId")
	query := `
    SELECT id, employee_id, consent_type, granted_at, revoked_at
    FROM consent_records
    WHERE tenant_id = $1
  `
	args := []any{user.TenantID}
	if employeeID != "" {
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	query += " ORDER BY granted_at DESC"

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "consent_list_failed", "failed to list consents", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var records []consentRecord
	for rows.Next() {
		var rec consentRecord
		if err := rows.Scan(&rec.ID, &rec.EmployeeID, &rec.ConsentType, &rec.GrantedAt, &rec.RevokedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "consent_list_failed", "failed to list consents", middleware.GetRequestID(r.Context()))
			return
		}
		records = append(records, rec)
	}
	api.Success(w, records, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateConsent(w http.ResponseWriter, r *http.Request) {
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
		EmployeeID  string `json:"employeeId"`
		ConsentType string `json:"consentType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.EmployeeID == "" || payload.ConsentType == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee id and consent type required", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	if err := h.DB.QueryRow(r.Context(), `
    INSERT INTO consent_records (tenant_id, employee_id, consent_type)
    VALUES ($1,$2,$3)
    RETURNING id
  `, user.TenantID, payload.EmployeeID, payload.ConsentType).Scan(&id); err != nil {
		api.Fail(w, http.StatusInternalServerError, "consent_create_failed", "failed to create consent", middleware.GetRequestID(r.Context()))
		return
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "gdpr.consent.create", "consent_record", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit gdpr.consent.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleRevokeConsent(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}
	consentID := chi.URLParam(r, "consentID")
	if _, err := h.DB.Exec(r.Context(), `
    UPDATE consent_records SET revoked_at = now()
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, consentID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "consent_revoke_failed", "failed to revoke consent", middleware.GetRequestID(r.Context()))
		return
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "gdpr.consent.revoke", "consent_record", consentID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		slog.Warn("audit gdpr.consent.revoke failed", "err", err)
	}
	api.Success(w, map[string]string{"status": "revoked"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListDSAR(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	query := `
    SELECT id, employee_id, requested_by, status, COALESCE(file_path,''), COALESCE(download_token,''), requested_at, completed_at
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

	page := shared.ParsePagination(r, 100, 500)
	countQuery := "SELECT COUNT(1) FROM dsar_exports WHERE tenant_id = $1"
	if len(args) == 2 {
		countQuery += " AND employee_id = $2"
	}
	var total int
	if err := h.DB.QueryRow(r.Context(), countQuery, args...).Scan(&total); err != nil {
		slog.Warn("dsar count failed", "err", err)
	}
	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", limitPos, offsetPos)
	args = append(args, page.Limit, page.Offset)

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "dsar_list_failed", "failed to list dsar exports", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var exports []map[string]any
	for rows.Next() {
		var id, employeeID, requestedBy, status, filePath, token string
		var requestedAt time.Time
		var completedAt any
		if err := rows.Scan(&id, &employeeID, &requestedBy, &status, &filePath, &token, &requestedAt, &completedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "dsar_list_failed", "failed to list dsar exports", middleware.GetRequestID(r.Context()))
			return
		}
		exports = append(exports, map[string]any{
			"id":            id,
			"employeeId":    employeeID,
			"requestedBy":   requestedBy,
			"status":        status,
			"filePath":      filePath,
			"downloadToken": token,
			"requestedAt":   requestedAt,
			"completedAt":   completedAt,
		})
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
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
	filePath, encrypted, token, err := h.generateDSAR(r.Context(), user.TenantID, payload.EmployeeID, id)
	if err == nil {
		status = gdpr.DSARStatusCompleted
		expiresAt := time.Now().Add(24 * time.Hour)
		if _, err := h.DB.Exec(r.Context(), `
      UPDATE dsar_exports
      SET status = $1,
          file_path = $2,
          file_encrypted = $3,
          download_token = $4,
          download_expires_at = $5,
          completed_at = now()
      WHERE id = $6
    `, gdpr.DSARStatusCompleted, filePath, encrypted, token, expiresAt, id); err != nil {
			slog.Warn("dsar complete update failed", "err", err)
		}
		if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "gdpr.dsar.complete", "dsar_export", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"filePath": filePath}); err != nil {
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
	var employeeID, filePath, token string
	var encrypted bool
	var tokenExpiry any
	err := h.DB.QueryRow(r.Context(), `
    SELECT employee_id, COALESCE(file_path, ''), COALESCE(download_token,''), file_encrypted, download_expires_at
    FROM dsar_exports
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, exportID).Scan(&employeeID, &filePath, &token, &encrypted, &tokenExpiry)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "dsar export not found", middleware.GetRequestID(r.Context()))
		return
	}

	requestToken := r.URL.Query().Get("token")
	if requestToken != "" {
		if token == "" || token != requestToken {
			api.Fail(w, http.StatusForbidden, "forbidden", "invalid download token", middleware.GetRequestID(r.Context()))
			return
		}
		if expiryTime, ok := tokenExpiry.(time.Time); ok && time.Now().After(expiryTime) {
			api.Fail(w, http.StatusForbidden, "forbidden", "download token expired", middleware.GetRequestID(r.Context()))
			return
		}
	} else if user.RoleName != auth.RoleHR {
		var selfEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
			slog.Warn("dsar download self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || selfEmployeeID != employeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if filePath == "" {
		api.Fail(w, http.StatusNotFound, "not_found", "dsar file not available", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := os.Stat(filePath); err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "dsar file not found", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=dsar-%s.json", exportID))
	if encrypted {
		if h.Crypto == nil || !h.Crypto.Configured() {
			api.Fail(w, http.StatusInternalServerError, "dsar_failed", "decryption unavailable", middleware.GetRequestID(r.Context()))
			return
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "dsar_failed", "failed to read dsar file", middleware.GetRequestID(r.Context()))
			return
		}
		plain, err := h.Crypto.Decrypt(data)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "dsar_failed", "failed to decrypt dsar file", middleware.GetRequestID(r.Context()))
			return
		}
		if _, err := w.Write(plain); err != nil {
			slog.Warn("dsar download write failed", "err", err)
		}
	} else {
		http.ServeFile(w, r, filePath)
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "gdpr.dsar.download", "dsar_export", exportID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		slog.Warn("audit gdpr.dsar.download failed", "err", err)
	}
}

func (h *Handler) generateDSAR(ctx context.Context, tenantID, employeeID, exportID string) (string, bool, string, error) {
	emp, err := h.Store.GetEmployee(ctx, tenantID, employeeID)
	if err != nil {
		return "", false, "", err
	}
	employeeJSON, err := json.Marshal(emp)
	if err != nil {
		return "", false, "", err
	}
	var employee map[string]any
	if err := json.Unmarshal(employeeJSON, &employee); err != nil {
		return "", false, "", err
	}

	queryRows := func(query string, args ...any) ([]map[string]any, error) {
		rows, err := h.DB.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []map[string]any
		for rows.Next() {
			var rowJSON []byte
			if err := rows.Scan(&rowJSON); err != nil {
				return nil, err
			}
			var row map[string]any
			if err := json.Unmarshal(rowJSON, &row); err != nil {
				return nil, err
			}
			out = append(out, row)
		}
		return out, rows.Err()
	}

	datasets := map[string]any{}
	if rows, err := queryRows(`SELECT row_to_json(lr) FROM leave_requests lr WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["leaveRequests"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(pr) FROM payroll_results pr WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["payrollResults"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(ps) FROM payslips ps WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["payslips"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(g) FROM goals g WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["goals"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(gc) FROM goal_comments gc JOIN goals g ON gc.goal_id = g.id WHERE g.tenant_id = $1 AND g.employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["goalComments"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(f) FROM feedback f WHERE tenant_id = $1 AND (to_employee_id = $2 OR from_user_id = $3)`, tenantID, employeeID, emp.UserID); err == nil {
		datasets["feedback"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(c) FROM checkins c WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["checkins"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(p) FROM pips p WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["pips"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(rt) FROM review_tasks rt WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["reviewTasks"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(rr) FROM review_responses rr JOIN review_tasks rt ON rr.task_id = rt.id WHERE rr.tenant_id = $1 AND rt.employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["reviewResponses"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(cr) FROM consent_records cr WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["consentRecords"] = rows
	}
	if emp.UserID != "" {
		if rows, err := queryRows(`SELECT row_to_json(n) FROM notifications n WHERE tenant_id = $1 AND user_id = $2`, tenantID, emp.UserID); err == nil {
			datasets["notifications"] = rows
		}
	}
	if rows, err := queryRows(`SELECT row_to_json(al) FROM access_logs al WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["accessLogs"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(mr) FROM manager_relations mr WHERE mr.employee_id = $1`, employeeID); err == nil {
		datasets["managerHistory"] = rows
	}

	payload := gdpr.BuildDSARPayload(employee, datasets)
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", false, "", err
	}

	if err := os.MkdirAll("storage/dsar", 0o755); err != nil {
		return "", false, "", err
	}

	encrypted := false
	filePath := filepath.Join("storage/dsar", exportID+".json")
	if h.Crypto != nil && h.Crypto.Configured() {
		enc, err := h.Crypto.Encrypt(jsonBytes)
		if err != nil {
			return "", false, "", err
		}
		encrypted = true
		filePath = filePath + ".enc"
		if err := os.WriteFile(filePath, enc, 0o600); err != nil {
			return "", false, "", err
		}
	} else {
		if err := os.WriteFile(filePath, jsonBytes, 0o600); err != nil {
			return "", false, "", err
		}
	}

	token, err := generateDownloadToken()
	if err != nil {
		return "", encrypted, "", err
	}
	return filePath, encrypted, token, nil
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
    SELECT id, employee_id, status, reason, requested_at, completed_at, COALESCE(file_path,''), COALESCE(download_token,'')
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
		if err := rows.Scan(&job.ID, &job.EmployeeID, &job.Status, &job.Reason, &job.RequestedAt, &job.CompletedAt, &job.FilePath, &job.DownloadToken); err != nil {
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
        national_id_enc = NULL,
        bank_account = NULL,
        bank_account_enc = NULL,
        salary = NULL,
        salary_enc = NULL,
        currency = COALESCE(currency, 'USD'),
        employment_type = NULL,
        department_id = NULL,
        manager_id = NULL,
        pay_group_id = NULL,
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
          mfa_secret_enc = NULL,
          mfa_enabled = false,
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

	report := map[string]any{
		"employeeId":  employeeID,
		"status":      gdpr.AnonymizationCompleted,
		"completedAt": time.Now(),
	}
	if err := os.MkdirAll("storage/anonymization", 0o755); err == nil {
		reportPath := filepath.Join("storage/anonymization", jobID+".json")
		reportBytes, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			slog.Warn("anonymization report marshal failed", "err", err)
			reportBytes = []byte("{}")
		}
		encrypted := false
		if h.Crypto != nil && h.Crypto.Configured() {
			if enc, err := h.Crypto.Encrypt(reportBytes); err == nil {
				reportPath = reportPath + ".enc"
				encrypted = true
				if err := os.WriteFile(reportPath, enc, 0o600); err == nil {
					token, tokenErr := generateDownloadToken()
					if tokenErr == nil {
						expiresAt := time.Now().Add(24 * time.Hour)
						if _, err := h.DB.Exec(ctx, `
            UPDATE anonymization_jobs
            SET file_path = $1, download_token = $2, download_expires_at = $3
            WHERE tenant_id = $4 AND id = $5
          `, reportPath, token, expiresAt, user.TenantID, jobID); err != nil {
							slog.Warn("anonymization report update failed", "err", err)
						}
					}
				}
			}
		}
		if !encrypted {
			if err := os.WriteFile(reportPath, reportBytes, 0o600); err == nil {
				token, tokenErr := generateDownloadToken()
				if tokenErr == nil {
					expiresAt := time.Now().Add(24 * time.Hour)
					if _, err := h.DB.Exec(ctx, `
          UPDATE anonymization_jobs
          SET file_path = $1, download_token = $2, download_expires_at = $3
          WHERE tenant_id = $4 AND id = $5
        `, reportPath, token, expiresAt, user.TenantID, jobID); err != nil {
						slog.Warn("anonymization report update failed", "err", err)
					}
				}
			}
		}
	}

	if err := audit.New(h.DB).Record(ctx, user.TenantID, user.UserID, "gdpr.anonymize.execute", "anonymization_job", jobID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"employeeId": employeeID}); err != nil {
		slog.Warn("audit gdpr.anonymize.execute failed", "err", err)
	}
	api.Success(w, map[string]string{"status": gdpr.AnonymizationCompleted}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleDownloadAnonymizationReport(w http.ResponseWriter, r *http.Request) {
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
	var filePath, token string
	var tokenExpiry any
	if err := h.DB.QueryRow(r.Context(), `
    SELECT COALESCE(file_path,''), COALESCE(download_token,''), download_expires_at
    FROM anonymization_jobs
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, jobID).Scan(&filePath, &token, &tokenExpiry); err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "anonymization report not found", middleware.GetRequestID(r.Context()))
		return
	}
	if filePath == "" {
		api.Fail(w, http.StatusNotFound, "not_found", "anonymization report not available", middleware.GetRequestID(r.Context()))
		return
	}
	requestToken := r.URL.Query().Get("token")
	if requestToken != "" {
		if token == "" || token != requestToken {
			api.Fail(w, http.StatusForbidden, "forbidden", "invalid download token", middleware.GetRequestID(r.Context()))
			return
		}
		if expiryTime, ok := tokenExpiry.(time.Time); ok && time.Now().After(expiryTime) {
			api.Fail(w, http.StatusForbidden, "forbidden", "download token expired", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if _, err := os.Stat(filePath); err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "anonymization report not found", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=anonymization-%s.json", jobID))
	if h.Crypto != nil && h.Crypto.Configured() && strings.HasSuffix(filePath, ".enc") {
		data, err := os.ReadFile(filePath)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to read report", middleware.GetRequestID(r.Context()))
			return
		}
		plain, err := h.Crypto.Decrypt(data)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to decrypt report", middleware.GetRequestID(r.Context()))
			return
		}
		if _, err := w.Write(plain); err != nil {
			slog.Warn("anonymization report write failed", "err", err)
		}
		return
	}

	http.ServeFile(w, r, filePath)
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

	page := shared.ParsePagination(r, 100, 500)
	var total int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM access_logs WHERE tenant_id = $1", user.TenantID).Scan(&total); err != nil {
		slog.Warn("access log count failed", "err", err)
	}
	rows, err := h.DB.Query(r.Context(), `
    SELECT id, actor_user_id, employee_id, fields, request_id, created_at
    FROM access_logs
    WHERE tenant_id = $1
    ORDER BY created_at DESC
    LIMIT $2 OFFSET $3
  `, user.TenantID, page.Limit, page.Offset)
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

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	api.Success(w, logs, middleware.GetRequestID(r.Context()))
}

func generateDownloadToken() (string, error) {
	buff := make([]byte, 32)
	if _, err := rand.Read(buff); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buff), nil
}
