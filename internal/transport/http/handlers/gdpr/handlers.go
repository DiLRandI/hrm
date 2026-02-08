package gdprhandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/domain/gdpr"
	cryptoutil "hrm/internal/platform/crypto"
	"hrm/internal/platform/jobs"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	Service *gdpr.Service
	Perms   middleware.PermissionStore
	Crypto  *cryptoutil.Service
	Jobs    *jobs.Service
	Audit   *audit.Service
}

var retentionDataCategories = []string{
	gdpr.DataCategoryAudit,
	gdpr.DataCategoryLeave,
	gdpr.DataCategoryPayroll,
	gdpr.DataCategoryPerformance,
	gdpr.DataCategoryGDPR,
	gdpr.DataCategoryAccessLogs,
	gdpr.DataCategoryNotifications,
	gdpr.DataCategoryProfile,
	gdpr.DataCategoryEmergency,
}

func NewHandler(service *gdpr.Service, perms middleware.PermissionStore, crypto *cryptoutil.Service, jobsSvc *jobs.Service, auditSvc *audit.Service) *Handler {
	return &Handler{Service: service, Perms: perms, Crypto: crypto, Jobs: jobsSvc, Audit: auditSvc}
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

func (h *Handler) handleListRetention(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	policies, err := h.Service.ListRetentionPolicies(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "retention_list_failed", "failed to list retention policies", middleware.GetRequestID(r.Context()))
		return
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

	var payload gdpr.RetentionPolicy
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "must be valid JSON"},
		})
		return
	}
	payload.DataCategory = strings.ToLower(strings.TrimSpace(payload.DataCategory))
	validator := shared.NewValidator()
	validator.Required("dataCategory", payload.DataCategory, "is required")
	validator.Enum("dataCategory", payload.DataCategory, retentionDataCategories, "must be one of the supported data categories")
	if payload.RetentionDays <= 0 {
		validator.Add("retentionDays", "must be greater than 0")
	}
	if payload.RetentionDays > 36500 {
		validator.Add("retentionDays", "must be less than or equal to 36500")
	}
	if validator.Reject(w, middleware.GetRequestID(r.Context())) {
		return
	}
	id, err := h.Service.UpsertRetentionPolicy(r.Context(), user.TenantID, payload.DataCategory, payload.RetentionDays)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "retention_create_failed", "failed to save retention policy", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "gdpr.retention.save", "retention_policy", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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

	runs, err := h.Service.ListRetentionRuns(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "retention_runs_failed", "failed to list retention runs", middleware.GetRequestID(r.Context()))
		return
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
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "must be valid JSON"},
		})
		return
	}
	payload.DataCategory = strings.ToLower(strings.TrimSpace(payload.DataCategory))
	validator := shared.NewValidator()
	if payload.DataCategory != "" {
		validator.Enum("dataCategory", payload.DataCategory, retentionDataCategories, "must be one of the supported data categories")
	}
	if validator.Reject(w, middleware.GetRequestID(r.Context())) {
		return
	}

	policies, err := h.Service.ListRetentionPolicies(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "retention_run_failed", "failed to load retention policies", middleware.GetRequestID(r.Context()))
		return
	}

	var summaries []gdpr.RetentionRunSummary
	for _, policy := range policies {
		category := policy.DataCategory
		retentionDays := policy.RetentionDays
		if payload.DataCategory != "" && payload.DataCategory != category {
			continue
		}
		if retentionDays <= 0 {
			continue
		}

		cutoff := time.Now().AddDate(0, 0, -retentionDays)
		runID := ""
		if h.Jobs != nil {
			if id, err := h.Service.CreateJobRun(r.Context(), user.TenantID, jobs.JobRetention); err != nil {
				slog.Warn("retention job run insert failed", "err", err)
			} else {
				runID = id
			}
		}

		deleted, err := h.Service.ApplyRetention(r.Context(), user.TenantID, category, cutoff)
		status := "completed"
		if err != nil {
			status = "failed"
		}

		if runID != "" {
			details := map[string]any{
				"dataCategory": category,
				"cutoffDate":   cutoff,
				"deleted":      deleted,
			}
			if err := h.Service.UpdateJobRun(r.Context(), runID, status, details); err != nil {
				slog.Warn("retention job run update failed", "err", err)
			}
		}

		if _, err := h.Service.RecordRetentionRun(r.Context(), user.TenantID, category, cutoff, status, deleted); err != nil {
			slog.Warn("retention run insert failed", "err", err)
		}

		summaries = append(summaries, gdpr.RetentionRunSummary{
			DataCategory: category,
			CutoffDate:   cutoff,
			Status:       status,
			DeletedCount: deleted,
		})
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "gdpr.retention.run", "retention_run", "", middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, summaries); err != nil {
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
	records, err := h.Service.ListConsents(r.Context(), user.TenantID, employeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "consent_list_failed", "failed to list consents", middleware.GetRequestID(r.Context()))
		return
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

	id, err := h.Service.CreateConsent(r.Context(), user.TenantID, payload.EmployeeID, payload.ConsentType)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "consent_create_failed", "failed to create consent", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "gdpr.consent.create", "consent_record", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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
	if err := h.Service.RevokeConsent(r.Context(), user.TenantID, consentID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "consent_revoke_failed", "failed to revoke consent", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "gdpr.consent.revoke", "consent_record", consentID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
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

	employeeID := ""
	if user.RoleName != auth.RoleHR {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("dsar list employee lookup failed", "err", err)
		} else {
			employeeID = id
		}
		if employeeID == "" {
			api.Success(w, []gdpr.DSARExport{}, middleware.GetRequestID(r.Context()))
			return
		}
	}

	page := shared.ParsePagination(r, 100, 500)
	exports, total, err := h.Service.ListDSARExports(r.Context(), user.TenantID, employeeID, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "dsar_list_failed", "failed to list dsar exports", middleware.GetRequestID(r.Context()))
		return
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
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "must be valid JSON"},
		})
		return
	}
	payload.EmployeeID = strings.TrimSpace(payload.EmployeeID)
	if payload.EmployeeID == "" {
		if id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID); err != nil {
			slog.Warn("dsar request employee lookup failed", "err", err)
		} else {
			payload.EmployeeID = id
		}
	}
	if payload.EmployeeID == "" {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "employeeId", Reason: "is required"},
		})
		return
	}
	if user.RoleName != auth.RoleHR {
		selfEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("dsar request self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || payload.EmployeeID != selfEmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	id, err := h.Service.CreateDSARExport(r.Context(), user.TenantID, payload.EmployeeID, user.UserID, gdpr.DSARStatusProcessing)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "dsar_request_failed", "failed to request dsar", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "gdpr.dsar.request", "dsar_export", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit gdpr.dsar.request failed", "err", err)
	}

	status := gdpr.DSARStatusProcessing
	filePath, encrypted, token, err := h.Service.GenerateDSAR(r.Context(), user.TenantID, payload.EmployeeID, id)
	if err == nil {
		status = gdpr.DSARStatusCompleted
		expiresAt := time.Now().Add(24 * time.Hour)
		if err := h.Service.UpdateDSARExport(r.Context(), id, gdpr.DSARStatusCompleted, filePath, encrypted, token, expiresAt); err != nil {
			slog.Warn("dsar complete update failed", "err", err)
		}
		if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "gdpr.dsar.complete", "dsar_export", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"filePath": filePath}); err != nil {
			slog.Warn("audit gdpr.dsar.complete failed", "err", err)
		}
	} else {
		status = gdpr.DSARStatusFailed
		if err := h.Service.UpdateDSARStatus(r.Context(), id, gdpr.DSARStatusFailed); err != nil {
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
	employeeID, filePath, token, encrypted, tokenExpiry, err := h.Service.DSARDownloadInfo(r.Context(), user.TenantID, exportID)
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
		selfEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
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
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "gdpr.dsar.download", "dsar_export", exportID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		slog.Warn("audit gdpr.dsar.download failed", "err", err)
	}
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

	jobs, err := h.Service.ListAnonymizationJobs(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "anonymize_list_failed", "failed to list anonymization jobs", middleware.GetRequestID(r.Context()))
		return
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
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "must be valid JSON"},
		})
		return
	}
	payload.EmployeeID = strings.TrimSpace(payload.EmployeeID)
	payload.Reason = strings.TrimSpace(payload.Reason)
	validator := shared.NewValidator()
	validator.Required("employeeId", payload.EmployeeID, "is required")
	validator.Required("reason", payload.Reason, "is required")
	if validator.Reject(w, middleware.GetRequestID(r.Context())) {
		return
	}

	id, err := h.Service.CreateAnonymizationJob(r.Context(), user.TenantID, payload.EmployeeID, gdpr.AnonymizationRequested, payload.Reason)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "anonymize_request_failed", "failed to request anonymization", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "gdpr.anonymize.request", "anonymization_job", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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
	result, err := h.Service.ExecuteAnonymization(r.Context(), user.TenantID, jobID)
	if err != nil {
		switch err {
		case gdpr.ErrAnonymizationNotFound:
			api.Fail(w, http.StatusNotFound, "not_found", "anonymization job not found", middleware.GetRequestID(r.Context()))
			return
		case gdpr.ErrAnonymizationBadState:
			api.Fail(w, http.StatusBadRequest, "invalid_state", "anonymization job is not in requested state", middleware.GetRequestID(r.Context()))
			return
		default:
			api.Fail(w, http.StatusInternalServerError, "anonymize_failed", "failed to anonymize employee", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "gdpr.anonymize.execute", "anonymization_job", jobID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"employeeId": result.EmployeeID}); err != nil {
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
	filePath, token, tokenExpiry, err := h.Service.AnonymizationReportInfo(r.Context(), user.TenantID, jobID)
	if err != nil {
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
	logs, total, err := h.Service.ListAccessLogs(r.Context(), user.TenantID, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "access_log_failed", "failed to list access logs", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	api.Success(w, logs, middleware.GetRequestID(r.Context()))
}
