package leavehandler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/domain/leave"
	"hrm/internal/domain/notifications"
	"hrm/internal/platform/jobs"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	Service *leave.Service
	Perms   middleware.PermissionStore
	Notify  *notifications.Service
	Audit   *audit.Service
	Jobs    *jobs.Service
}

const (
	maxLeaveRequestDocuments      = 5
	maxLeaveRequestDocumentBytes  = 2 * 1024 * 1024
	maxLeaveRequestMultipartBytes = 8 * 1024 * 1024
)

func NewHandler(service *leave.Service, perms middleware.PermissionStore, notify *notifications.Service, auditSvc *audit.Service, jobsSvc *jobs.Service) *Handler {
	return &Handler{Service: service, Perms: perms, Notify: notify, Audit: auditSvc, Jobs: jobsSvc}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/leave", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/types", h.handleListTypes)
		r.With(middleware.RequirePermission(auth.PermLeaveWrite, h.Perms)).Post("/types", h.handleCreateType)
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/policies", h.handleListPolicies)
		r.With(middleware.RequirePermission(auth.PermLeaveWrite, h.Perms)).Post("/policies", h.handleCreatePolicy)
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/holidays", h.handleListHolidays)
		r.With(middleware.RequirePermission(auth.PermLeaveWrite, h.Perms)).Post("/holidays", h.handleCreateHoliday)
		r.With(middleware.RequirePermission(auth.PermLeaveWrite, h.Perms)).Delete("/holidays/{holidayID}", h.handleDeleteHoliday)
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/balances", h.handleListBalances)
		r.With(middleware.RequirePermission(auth.PermLeaveWrite, h.Perms)).Post("/balances/adjust", h.handleAdjustBalance)
		r.With(middleware.RequirePermission(auth.PermLeaveWrite, h.Perms)).Post("/accrual/run", h.handleRunAccruals)
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/requests", h.handleListRequests)
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/requests/{requestID}", h.handleGetRequest)
		r.With(middleware.RequirePermission(auth.PermLeaveWrite, h.Perms)).Post("/requests", h.handleCreateRequest)
		r.With(middleware.RequirePermission(auth.PermLeaveWrite, h.Perms)).Post("/requests/{requestID}/documents", h.handleUploadRequestDocument)
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/requests/{requestID}/documents/{documentID}/download", h.handleDownloadRequestDocument)
		r.With(middleware.RequirePermission(auth.PermLeaveApprove, h.Perms)).Post("/requests/{requestID}/approve", h.handleApproveRequest)
		r.With(middleware.RequirePermission(auth.PermLeaveApprove, h.Perms)).Post("/requests/{requestID}/reject", h.handleRejectRequest)
		r.With(middleware.RequirePermission(auth.PermLeaveWrite, h.Perms)).Post("/requests/{requestID}/cancel", h.handleCancelRequest)
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/calendar", h.handleCalendar)
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/calendar/export", h.handleCalendarExport)
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/reports/balances", h.handleReportBalances)
		r.With(middleware.RequirePermission(auth.PermLeaveRead, h.Perms)).Get("/reports/usage", h.handleReportUsage)
	})
}

func (h *Handler) handleListTypes(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	types, err := h.Service.ListTypes(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_types_failed", "failed to list leave types", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, types, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateType(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload leave.LeaveType
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	id, err := h.Service.CreateType(r.Context(), user.TenantID, payload)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_type_create_failed", "failed to create leave type", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.type.create", "leave_type", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit leave.type.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	policies, err := h.Service.ListPolicies(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_policies_failed", "failed to list leave policies", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, policies, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload leave.LeavePolicy
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	id, err := h.Service.CreatePolicy(r.Context(), user.TenantID, payload)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_policy_create_failed", "failed to create leave policy", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.policy.create", "leave_policy", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit leave.policy.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

type holidayPayload struct {
	Date   string `json:"date"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

func (h *Handler) handleListHolidays(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	out, err := h.Service.ListHolidays(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "holiday_list_failed", "failed to list holidays", middleware.GetRequestID(r.Context()))
		return
	}

	api.Success(w, out, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateHoliday(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload holidayPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	holidayDate, err := shared.ParseDate(payload.Date)
	if err != nil || holidayDate.IsZero() {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid date", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.Name == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "name required", middleware.GetRequestID(r.Context()))
		return
	}

	id, err := h.Service.CreateHoliday(r.Context(), user.TenantID, holidayDate, payload.Name, payload.Region)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "holiday_create_failed", "failed to create holiday", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.holiday.create", "holiday", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit leave.holiday.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleDeleteHoliday(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	holidayID := chi.URLParam(r, "holidayID")
	if err := h.Service.DeleteHoliday(r.Context(), user.TenantID, holidayID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "holiday_delete_failed", "failed to delete holiday", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.holiday.delete", "holiday", holidayID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		slog.Warn("audit leave.holiday.delete failed", "err", err)
	}
	api.Success(w, map[string]string{"status": "deleted"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListBalances(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID := r.URL.Query().Get("employeeId")
	var selfEmployeeID string
	if user.RoleName != auth.RoleHR || employeeID == "" {
		if id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID); err == nil {
			selfEmployeeID = id
		} else {
			slog.Warn("leave balances self employee lookup failed", "err", err)
		}
	}

	switch user.RoleName {
	case auth.RoleEmployee:
		employeeID = selfEmployeeID
	case auth.RoleManager:
		if employeeID == "" {
			employeeID = selfEmployeeID
		}
		if employeeID != "" && employeeID != selfEmployeeID {
			allowed, err := h.Service.IsManagerOf(r.Context(), user.TenantID, selfEmployeeID, employeeID)
			if err != nil {
				slog.Warn("leave balances manager scope check failed", "err", err)
			}
			if !allowed {
				api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
				return
			}
		}
	case auth.RoleHR:
		if employeeID == "" {
			api.Fail(w, http.StatusBadRequest, "invalid_request", "employee id required", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if employeeID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_request", "employee id required", middleware.GetRequestID(r.Context()))
		return
	}

	balances, err := h.Service.ListBalances(r.Context(), user.TenantID, employeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_balances_failed", "failed to list balances", middleware.GetRequestID(r.Context()))
		return
	}

	api.Success(w, balances, middleware.GetRequestID(r.Context()))
}

type adjustBalanceRequest struct {
	EmployeeID  string  `json:"employeeId"`
	LeaveTypeID string  `json:"leaveTypeId"`
	Amount      float64 `json:"amount"`
	Reason      string  `json:"reason"`
}

func (h *Handler) handleAdjustBalance(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload adjustBalanceRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.AdjustBalance(r.Context(), user.TenantID, payload.EmployeeID, payload.LeaveTypeID, payload.Reason, user.UserID, payload.Amount); err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_adjust_failed", "failed to adjust balance", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.balance.adjust", "leave_balance", payload.EmployeeID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit leave.balance.adjust failed", "err", err)
	}
	api.Success(w, map[string]string{"status": "adjusted"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleRunAccruals(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	now := time.Now()
	summary := leave.AccrualSummary{}
	var err error
	if h.Jobs != nil {
		result, runErr := h.Jobs.RunNow(r.Context(), jobs.JobLeaveAccrual, user.TenantID, func(runCtx context.Context) (any, error) {
			return h.Service.RunAccruals(runCtx, user.TenantID, now)
		})
		err = runErr
		if result != nil {
			if s, ok := result.(leave.AccrualSummary); ok {
				summary = s
			} else if m, ok := result.(map[string]any); ok {
				if policies, ok := m["PoliciesProcessed"].(int); ok {
					summary.PoliciesProcessed = policies
				}
			}
		}
	} else {
		summary, err = h.Service.RunAccruals(r.Context(), user.TenantID, now)
	}
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "accrual_failed", "failed to run accruals", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.accrual.run", "leave_policy", "", middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, summary); err != nil {
		slog.Warn("audit leave.accrual.run failed", "err", err)
	}
	api.Success(w, summary, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListRequests(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	var employeeID string
	var managerEmployeeID string
	if user.RoleName == auth.RoleEmployee {
		if id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID); err == nil {
			employeeID = id
		} else {
			slog.Warn("leave requests employee lookup failed", "err", err)
		}
	}
	if user.RoleName == auth.RoleManager {
		if id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID); err == nil {
			managerEmployeeID = id
		} else {
			slog.Warn("leave requests manager lookup failed", "err", err)
		}
	}

	page := shared.ParsePagination(r, 100, 500)
	result, err := h.Service.ListRequests(r.Context(), user.TenantID, user.RoleName, employeeID, managerEmployeeID, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_requests_failed", "failed to list requests", middleware.GetRequestID(r.Context()))
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(result.Total))
	api.Success(w, result.Requests, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	requestID := chi.URLParam(r, "requestID")
	req, err := h.Service.GetRequest(r.Context(), user.TenantID, requestID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "leave request not found", middleware.GetRequestID(r.Context()))
		return
	}

	allowed, err := h.canAccessRequest(r.Context(), user, req.EmployeeID)
	if err != nil {
		slog.Warn("leave request access check failed", "requestId", requestID, "err", err)
	}
	if !allowed {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
		return
	}

	api.Success(w, req, middleware.GetRequestID(r.Context()))
}

type leaveRequestPayload struct {
	EmployeeID  string `json:"employeeId"`
	LeaveTypeID string `json:"leaveTypeId"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	StartHalf   bool   `json:"startHalf"`
	EndHalf     bool   `json:"endHalf"`
	Reason      string `json:"reason"`
}

func decodeLeaveRequestPayload(r *http.Request) (leaveRequestPayload, []leave.LeaveRequestDocumentUpload, error) {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxLeaveRequestMultipartBytes); err != nil {
			return leaveRequestPayload{}, nil, fmt.Errorf("invalid multipart payload")
		}

		startHalf, err := parseOptionalBool(r.FormValue("startHalf"))
		if err != nil {
			return leaveRequestPayload{}, nil, fmt.Errorf("invalid startHalf value")
		}
		endHalf, err := parseOptionalBool(r.FormValue("endHalf"))
		if err != nil {
			return leaveRequestPayload{}, nil, fmt.Errorf("invalid endHalf value")
		}

		documents, err := parseMultipartDocuments(r.MultipartForm.File["documents"])
		if err != nil {
			return leaveRequestPayload{}, nil, err
		}

		return leaveRequestPayload{
			EmployeeID:  strings.TrimSpace(r.FormValue("employeeId")),
			LeaveTypeID: strings.TrimSpace(r.FormValue("leaveTypeId")),
			StartDate:   strings.TrimSpace(r.FormValue("startDate")),
			EndDate:     strings.TrimSpace(r.FormValue("endDate")),
			StartHalf:   startHalf,
			EndHalf:     endHalf,
			Reason:      strings.TrimSpace(r.FormValue("reason")),
		}, documents, nil
	}

	var payload leaveRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return leaveRequestPayload{}, nil, err
	}
	return payload, nil, nil
}

func parseMultipartDocuments(files []*multipart.FileHeader) ([]leave.LeaveRequestDocumentUpload, error) {
	if len(files) > maxLeaveRequestDocuments {
		return nil, fmt.Errorf("too many documents")
	}

	documents := make([]leave.LeaveRequestDocumentUpload, 0, len(files))
	for _, header := range files {
		if header == nil {
			continue
		}
		if header.Size > maxLeaveRequestDocumentBytes {
			return nil, fmt.Errorf("document exceeds maximum size")
		}

		file, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open document")
		}
		content, err := io.ReadAll(io.LimitReader(file, maxLeaveRequestDocumentBytes+1))
		closeErr := file.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read document")
		}
		if closeErr != nil {
			return nil, fmt.Errorf("failed to close document")
		}
		if int64(len(content)) > maxLeaveRequestDocumentBytes {
			return nil, fmt.Errorf("document exceeds maximum size")
		}
		if len(content) == 0 {
			return nil, fmt.Errorf("empty document is not allowed")
		}

		fileName := sanitizeUploadedFileName(header.Filename)
		contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = http.DetectContentType(content)
		}

		documents = append(documents, leave.LeaveRequestDocumentUpload{
			FileName:    fileName,
			ContentType: contentType,
			FileSize:    int64(len(content)),
			Data:        content,
		})
	}

	return documents, nil
}

func parseOptionalBool(raw string) (bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, err
	}
	return parsed, nil
}

func sanitizeUploadedFileName(name string) string {
	cleaned := strings.TrimSpace(filepath.Base(name))
	cleaned = strings.ReplaceAll(cleaned, "\x00", "")
	if cleaned == "" {
		return "document.bin"
	}
	return cleaned
}

func (h *Handler) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	payload, documents, err := decodeLeaveRequestPayload(r)
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", err.Error(), middleware.GetRequestID(r.Context()))
		return
	}
	if payload.LeaveTypeID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "leave type required", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName != auth.RoleHR {
		if id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID); err == nil {
			payload.EmployeeID = id
		} else {
			slog.Warn("leave request self employee lookup failed", "err", err)
		}
	}
	if strings.TrimSpace(payload.EmployeeID) == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee id required", middleware.GetRequestID(r.Context()))
		return
	}

	startDate, err := shared.ParseDate(payload.StartDate)
	if err != nil || startDate.IsZero() {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid start date", middleware.GetRequestID(r.Context()))
		return
	}
	endDate, err := shared.ParseDate(payload.EndDate)
	if err != nil || endDate.IsZero() {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid end date", middleware.GetRequestID(r.Context()))
		return
	}

	days, err := leave.CalculateRequestDays(startDate, endDate, payload.StartHalf, payload.EndHalf)
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_dates", "invalid date or half-day range", middleware.GetRequestID(r.Context()))
		return
	}

	requiresDoc, err := h.Service.LeaveTypeRequiresDoc(r.Context(), user.TenantID, payload.LeaveTypeID)
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid leave type", middleware.GetRequestID(r.Context()))
		return
	}
	if requiresDoc && len(documents) == 0 {
		api.Fail(w, http.StatusBadRequest, "document_required", "supporting document is required for this leave type", middleware.GetRequestID(r.Context()))
		return
	}

	result, err := h.Service.CreateRequest(r.Context(), user.TenantID, payload.EmployeeID, payload.LeaveTypeID, payload.Reason, startDate, endDate, payload.StartHalf, payload.EndHalf, days)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_request_failed", "failed to create request", middleware.GetRequestID(r.Context()))
		return
	}

	createdDocs := make([]leave.LeaveRequestDocument, 0, len(documents))
	for _, doc := range documents {
		createdDoc, err := h.Service.CreateRequestDocument(r.Context(), user.TenantID, result.ID, doc, user.UserID)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "leave_request_failed", "failed to store supporting document", middleware.GetRequestID(r.Context()))
			return
		}
		createdDocs = append(createdDocs, createdDoc)
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.request.create", "leave_request", result.ID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{
		"employeeId":    payload.EmployeeID,
		"leaveTypeId":   payload.LeaveTypeID,
		"startDate":     payload.StartDate,
		"endDate":       payload.EndDate,
		"startHalf":     payload.StartHalf,
		"endHalf":       payload.EndHalf,
		"reason":        payload.Reason,
		"days":          days,
		"documentCount": len(createdDocs),
	}); err != nil {
		slog.Warn("audit leave.request.create failed", "err", err)
	}
	if result.ManagerUserID != "" {
		if h.Notify != nil {
			if err := h.Notify.Create(r.Context(), user.TenantID, result.ManagerUserID, notifications.TypeLeaveSubmitted, "Leave request submitted", "A leave request is awaiting approval."); err != nil {
				slog.Warn("leave submitted notification failed", "err", err)
			}
		}
	}
	if len(result.HRUserIDs) > 0 && h.Notify != nil {
		for _, hrUserID := range result.HRUserIDs {
			if err := h.Notify.Create(r.Context(), user.TenantID, hrUserID, notifications.TypeLeaveSubmitted, "Leave request awaiting HR", "A leave request is awaiting HR approval."); err != nil {
				slog.Warn("leave hr notification failed", "err", err)
			}
		}
	}

	api.Created(w, map[string]any{
		"id":          result.ID,
		"status":      result.Status,
		"days":        days,
		"startHalf":   payload.StartHalf,
		"endHalf":     payload.EndHalf,
		"documents":   createdDocs,
		"requiresDoc": requiresDoc,
	}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleUploadRequestDocument(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	requestID := chi.URLParam(r, "requestID")
	req, err := h.Service.GetRequest(r.Context(), user.TenantID, requestID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "leave request not found", middleware.GetRequestID(r.Context()))
		return
	}

	allowed, err := h.canAccessRequest(r.Context(), user, req.EmployeeID)
	if err != nil {
		slog.Warn("leave request document access check failed", "requestId", requestID, "err", err)
	}
	if !allowed {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
		return
	}

	if err := r.ParseMultipartForm(maxLeaveRequestMultipartBytes); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid multipart payload", middleware.GetRequestID(r.Context()))
		return
	}
	documents, err := parseMultipartDocuments(r.MultipartForm.File["documents"])
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", err.Error(), middleware.GetRequestID(r.Context()))
		return
	}
	if len(documents) == 0 {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "at least one document is required", middleware.GetRequestID(r.Context()))
		return
	}

	createdDocs := make([]leave.LeaveRequestDocument, 0, len(documents))
	for _, doc := range documents {
		createdDoc, err := h.Service.CreateRequestDocument(r.Context(), user.TenantID, requestID, doc, user.UserID)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "leave_document_upload_failed", "failed to upload document", middleware.GetRequestID(r.Context()))
			return
		}
		createdDocs = append(createdDocs, createdDoc)
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.request.document.upload", "leave_request", requestID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{
		"documentCount": len(createdDocs),
	}); err != nil {
		slog.Warn("audit leave.request.document.upload failed", "err", err)
	}

	api.Created(w, map[string]any{"documents": createdDocs}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleDownloadRequestDocument(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	requestID := chi.URLParam(r, "requestID")
	documentID := chi.URLParam(r, "documentID")
	req, err := h.Service.GetRequest(r.Context(), user.TenantID, requestID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "leave request not found", middleware.GetRequestID(r.Context()))
		return
	}

	allowed, err := h.canAccessRequest(r.Context(), user, req.EmployeeID)
	if err != nil {
		slog.Warn("leave request document access check failed", "requestId", requestID, "err", err)
	}
	if !allowed {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
		return
	}

	document, data, err := h.Service.RequestDocumentData(r.Context(), user.TenantID, requestID, documentID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "document not found", middleware.GetRequestID(r.Context()))
		return
	}

	contentType := document.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", document.FileName))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		slog.Warn("leave document download write failed", "requestId", requestID, "documentId", documentID, "err", err)
	}
}

func (h *Handler) canAccessRequest(ctx context.Context, user auth.UserContext, requestEmployeeID string) (bool, error) {
	if user.RoleName == auth.RoleHR {
		return true, nil
	}

	selfEmployeeID, err := h.Service.EmployeeIDByUserID(ctx, user.TenantID, user.UserID)
	if err != nil {
		return false, err
	}
	if selfEmployeeID == "" {
		return false, nil
	}

	if user.RoleName == auth.RoleEmployee {
		return selfEmployeeID == requestEmployeeID, nil
	}
	if user.RoleName == auth.RoleManager {
		if selfEmployeeID == requestEmployeeID {
			return true, nil
		}
		allowed, err := h.Service.IsManagerOf(ctx, user.TenantID, selfEmployeeID, requestEmployeeID)
		if err != nil {
			return false, err
		}
		return allowed, nil
	}

	return false, nil
}

func (h *Handler) handleApproveRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleManager && user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "manager or hr required", middleware.GetRequestID(r.Context()))
		return
	}

	requestID := chi.URLParam(r, "requestID")
	result, err := h.Service.ApproveRequest(r.Context(), user.TenantID, requestID, user.UserID, user.RoleName)
	if err != nil {
		if errors.Is(err, leave.ErrHRApprovalRequired) {
			api.Fail(w, http.StatusForbidden, "forbidden", "hr approval required", middleware.GetRequestID(r.Context()))
			return
		}
		if errors.Is(err, leave.ErrForbidden) {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
		api.Fail(w, http.StatusNotFound, "not_found", "leave request not found", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.request.approve", "leave_request", requestID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"employeeId": result.EmployeeID}); err != nil {
		slog.Warn("audit leave.request.approve failed", "err", err)
	}

	if !result.FinalApproval && h.Notify != nil {
		for _, hrUserID := range result.HRUserIDs {
			if err := h.Notify.Create(r.Context(), user.TenantID, hrUserID, notifications.TypeLeaveSubmitted, "Leave request awaiting HR", "A leave request is awaiting HR approval."); err != nil {
				slog.Warn("leave hr notification failed", "err", err)
			}
		}
	}

	if result.EmployeeUser != "" && result.Status == leave.StatusApproved {
		title := "Leave approved"
		body := fmt.Sprintf("Your %s leave request was approved.", result.LeaveTypeName)
		if h.Notify != nil {
			if err := h.Notify.Create(r.Context(), user.TenantID, result.EmployeeUser, notifications.TypeLeaveApproved, title, body); err != nil {
				slog.Warn("leave approved notification failed", "err", err)
			}
		}
	}

	api.Success(w, map[string]string{"status": result.Status}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleRejectRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleManager && user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "manager or hr required", middleware.GetRequestID(r.Context()))
		return
	}

	requestID := chi.URLParam(r, "requestID")
	result, err := h.Service.RejectRequest(r.Context(), user.TenantID, requestID, user.UserID, user.RoleName)
	if err != nil {
		if errors.Is(err, leave.ErrForbidden) {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
		api.Fail(w, http.StatusNotFound, "not_found", "leave request not found", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.request.reject", "leave_request", requestID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"employeeId": result.EmployeeID}); err != nil {
		slog.Warn("audit leave.request.reject failed", "err", err)
	}
	if result.EmployeeUser != "" {
		title := "Leave rejected"
		body := fmt.Sprintf("Your %s leave request was rejected.", result.LeaveTypeName)
		if h.Notify != nil {
			if err := h.Notify.Create(r.Context(), user.TenantID, result.EmployeeUser, notifications.TypeLeaveRejected, title, body); err != nil {
				slog.Warn("leave rejected notification failed", "err", err)
			}
		}
	}

	api.Success(w, map[string]string{"status": leave.StatusRejected}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCancelRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	requestID := chi.URLParam(r, "requestID")
	result, err := h.Service.CancelRequest(r.Context(), user.TenantID, requestID, user.UserID)
	if err != nil {
		if errors.Is(err, leave.ErrInvalidState) {
			api.Fail(w, http.StatusBadRequest, "invalid_state", "approved requests require HR cancellation", middleware.GetRequestID(r.Context()))
			return
		}
		api.Fail(w, http.StatusNotFound, "not_found", "leave request not found", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "leave.request.cancel", "leave_request", requestID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"employeeId": result.EmployeeID}); err != nil {
		slog.Warn("audit leave.request.cancel failed", "err", err)
	}
	if result.EmployeeUser != "" {
		title := "Leave cancelled"
		body := fmt.Sprintf("Your %s leave request was cancelled.", result.LeaveTypeName)
		if h.Notify != nil {
			if err := h.Notify.Create(r.Context(), user.TenantID, result.EmployeeUser, notifications.TypeLeaveCancelled, title, body); err != nil {
				slog.Warn("leave cancelled notification failed", "err", err)
			}
		}
	}

	api.Success(w, map[string]string{"status": leave.StatusCancelled}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCalendar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	entries, err := h.Service.CalendarEntries(r.Context(), user.TenantID, user.RoleName, user.UserID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "calendar_failed", "failed to load calendar", middleware.GetRequestID(r.Context()))
		return
	}

	var events []map[string]any
	for _, entry := range entries {
		events = append(events, map[string]any{
			"id":          entry.ID,
			"employeeId":  entry.EmployeeID,
			"leaveTypeId": entry.LeaveTypeID,
			"start":       entry.StartDate,
			"end":         entry.EndDate,
			"status":      entry.Status,
		})
	}
	api.Success(w, events, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCalendarExport(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "csv"
	}

	rows, err := h.Service.CalendarExportRows(r.Context(), user.TenantID, user.RoleName, user.UserID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "calendar_failed", "failed to load calendar", middleware.GetRequestID(r.Context()))
		return
	}

	if format == "ics" {
		w.Header().Set("Content-Type", "text/calendar")
		w.Header().Set("Content-Disposition", "attachment; filename=leave-calendar.ics")
		var builder strings.Builder
		builder.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//PulseHR//Leave Calendar//EN\r\n")
		for _, row := range rows {
			builder.WriteString("BEGIN:VEVENT\r\n")
			builder.WriteString(fmt.Sprintf("UID:%s\r\n", row.ID))
			builder.WriteString(fmt.Sprintf("DTSTART:%s\r\n", row.StartDate.Format("20060102")))
			builder.WriteString(fmt.Sprintf("DTEND:%s\r\n", row.EndDate.AddDate(0, 0, 1).Format("20060102")))
			builder.WriteString(fmt.Sprintf("SUMMARY:%s (%s)\r\n", row.LeaveTypeName, row.Status))
			builder.WriteString("END:VEVENT\r\n")
		}
		builder.WriteString("END:VCALENDAR\r\n")
		if _, err := w.Write([]byte(builder.String())); err != nil {
			slog.Warn("calendar export write failed", "err", err)
		}
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=leave-calendar.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"id", "employee_id", "leave_type", "start_date", "end_date", "status"}); err != nil {
		slog.Warn("calendar export csv header write failed", "err", err)
	}
	for _, row := range rows {
		if err := writer.Write([]string{row.ID, row.EmployeeID, row.LeaveTypeName, row.StartDate.Format("2006-01-02"), row.EndDate.Format("2006-01-02"), row.Status}); err != nil {
			slog.Warn("calendar export csv row write failed", "err", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		slog.Warn("calendar export csv flush failed", "err", err)
	}
}

func (h *Handler) handleReportBalances(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	report, err := h.Service.ReportBalances(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "report_failed", "failed to load report", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, report, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleReportUsage(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	report, err := h.Service.ReportUsage(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "report_failed", "failed to load report", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, report, middleware.GetRequestID(r.Context()))
}
