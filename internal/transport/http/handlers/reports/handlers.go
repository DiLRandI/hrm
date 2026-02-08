package reportshandler

import (
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/reports"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	Service *reports.Service
	Perms   middleware.PermissionStore
}

func NewHandler(service *reports.Service, perms middleware.PermissionStore) *Handler {
	return &Handler{Service: service, Perms: perms}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/reports", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/employee", h.handleEmployeeDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/manager", h.handleManagerDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/hr", h.handleHRDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/employee/export", h.handleExportEmployeeDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/manager/export", h.handleExportManagerDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/hr/export", h.handleExportHRDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/jobs", h.handleJobRuns)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/jobs/{runID}", h.handleJobRunDetail)
	})
}

func (h *Handler) handleEmployeeDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleEmployee && user.RoleName != auth.RoleManager && user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
	if err != nil {
		slog.Warn("employee lookup failed", "err", err)
	}

	var leaveBalance float64
	if employeeID != "" {
		leaveBalance, err = h.Service.LeaveBalance(r.Context(), user.TenantID, employeeID)
		if err != nil {
			slog.Warn("leave balance aggregate failed", "err", err)
		}
	}
	if employeeID == "" {
		leaveBalance = 0
	}

	var payslipCount int
	if employeeID != "" {
		payslipCount, err = h.Service.PayslipCount(r.Context(), user.TenantID, employeeID)
		if err != nil {
			slog.Warn("payslip count failed", "err", err)
		}
	}

	var goalCount int
	if employeeID != "" {
		goalCount, err = h.Service.GoalCount(r.Context(), user.TenantID, employeeID)
		if err != nil {
			slog.Warn("goal count failed", "err", err)
		}
	}
	if employeeID == "" {
		payslipCount = 0
		goalCount = 0
	}

	api.Success(w, reports.EmployeeDashboard(leaveBalance, payslipCount, goalCount), middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleManagerDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleManager && user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "manager or hr required", middleware.GetRequestID(r.Context()))
		return
	}

	managerEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
	if err != nil {
		slog.Warn("manager employee lookup failed", "err", err)
	}

	var pendingApprovals int
	pendingApprovals, err = h.Service.PendingApprovals(r.Context(), user.TenantID)
	if err != nil {
		slog.Warn("pending approvals count failed", "err", err)
	}

	var teamGoals int
	if managerEmployeeID != "" {
		teamGoals, err = h.Service.TeamGoals(r.Context(), user.TenantID, managerEmployeeID)
		if err != nil {
			slog.Warn("team goals count failed", "err", err)
		}
	}
	if managerEmployeeID == "" {
		teamGoals = 0
	}

	var reviewTasks int
	if managerEmployeeID != "" {
		reviewTasks, err = h.Service.ReviewTasks(r.Context(), user.TenantID, managerEmployeeID)
		if err != nil {
			slog.Warn("review tasks count failed", "err", err)
		}
	}
	if managerEmployeeID == "" {
		reviewTasks = 0
	}

	api.Success(w, reports.ManagerDashboard(pendingApprovals, teamGoals, reviewTasks), middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleHRDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payrollPeriods int
	payrollPeriods, err := h.Service.PayrollPeriods(r.Context(), user.TenantID)
	if err != nil {
		slog.Warn("payroll period count failed", "err", err)
	}

	var leavePending int
	leavePending, err = h.Service.LeavePending(r.Context(), user.TenantID)
	if err != nil {
		slog.Warn("leave pending count failed", "err", err)
	}

	var reviewCycles int
	reviewCycles, err = h.Service.ReviewCycles(r.Context(), user.TenantID)
	if err != nil {
		slog.Warn("review cycles count failed", "err", err)
	}

	api.Success(w, reports.HRDashboard(payrollPeriods, leavePending, reviewCycles), middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleExportEmployeeDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	employeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "export_failed", "failed to export dashboard", middleware.GetRequestID(r.Context()))
		return
	}

	var leaveBalance float64
	leaveBalance, err = h.Service.LeaveBalance(r.Context(), user.TenantID, employeeID)
	if err != nil {
		slog.Warn("leave balance aggregate failed", "err", err)
	}
	var payslipCount int
	payslipCount, err = h.Service.PayslipCount(r.Context(), user.TenantID, employeeID)
	if err != nil {
		slog.Warn("payslip count failed", "err", err)
	}
	var goalCount int
	goalCount, err = h.Service.GoalCount(r.Context(), user.TenantID, employeeID)
	if err != nil {
		slog.Warn("goal count failed", "err", err)
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=employee-dashboard.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"metric", "value"}); err != nil {
		slog.Warn("employee dashboard export header failed", "err", err)
	}
	if err := writer.Write([]string{"leave_balance", fmt.Sprintf("%.2f", leaveBalance)}); err != nil {
		slog.Warn("employee dashboard export row failed", "err", err)
	}
	if err := writer.Write([]string{"payslip_count", strconv.Itoa(payslipCount)}); err != nil {
		slog.Warn("employee dashboard export row failed", "err", err)
	}
	if err := writer.Write([]string{"goal_count", strconv.Itoa(goalCount)}); err != nil {
		slog.Warn("employee dashboard export row failed", "err", err)
	}
	writer.Flush()
}

func (h *Handler) handleExportManagerDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	managerEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "export_failed", "failed to export dashboard", middleware.GetRequestID(r.Context()))
		return
	}

	var pendingApprovals int
	pendingApprovals, err = h.Service.PendingApprovals(r.Context(), user.TenantID)
	if err != nil {
		slog.Warn("pending approvals count failed", "err", err)
	}
	var teamGoals int
	teamGoals, err = h.Service.TeamGoals(r.Context(), user.TenantID, managerEmployeeID)
	if err != nil {
		slog.Warn("team goals count failed", "err", err)
	}
	var reviewTasks int
	reviewTasks, err = h.Service.ReviewTasks(r.Context(), user.TenantID, managerEmployeeID)
	if err != nil {
		slog.Warn("review tasks count failed", "err", err)
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=manager-dashboard.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"metric", "value"}); err != nil {
		slog.Warn("manager dashboard export header failed", "err", err)
	}
	if err := writer.Write([]string{"pending_leave_approvals", strconv.Itoa(pendingApprovals)}); err != nil {
		slog.Warn("manager dashboard export row failed", "err", err)
	}
	if err := writer.Write([]string{"team_goals", strconv.Itoa(teamGoals)}); err != nil {
		slog.Warn("manager dashboard export row failed", "err", err)
	}
	if err := writer.Write([]string{"review_tasks", strconv.Itoa(reviewTasks)}); err != nil {
		slog.Warn("manager dashboard export row failed", "err", err)
	}
	writer.Flush()
}

func (h *Handler) handleExportHRDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var err error
	var payrollPeriods int
	payrollPeriods, err = h.Service.PayrollPeriods(r.Context(), user.TenantID)
	if err != nil {
		slog.Warn("payroll period count failed", "err", err)
	}
	var leavePending int
	leavePending, err = h.Service.LeavePending(r.Context(), user.TenantID)
	if err != nil {
		slog.Warn("leave pending count failed", "err", err)
	}
	var reviewCycles int
	reviewCycles, err = h.Service.ReviewCycles(r.Context(), user.TenantID)
	if err != nil {
		slog.Warn("review cycles count failed", "err", err)
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=hr-dashboard.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"metric", "value"}); err != nil {
		slog.Warn("hr dashboard export header failed", "err", err)
	}
	if err := writer.Write([]string{"payroll_periods", strconv.Itoa(payrollPeriods)}); err != nil {
		slog.Warn("hr dashboard export row failed", "err", err)
	}
	if err := writer.Write([]string{"leave_pending", strconv.Itoa(leavePending)}); err != nil {
		slog.Warn("hr dashboard export row failed", "err", err)
	}
	if err := writer.Write([]string{"review_cycles", strconv.Itoa(reviewCycles)}); err != nil {
		slog.Warn("hr dashboard export row failed", "err", err)
	}
	writer.Flush()
}

func (h *Handler) handleJobRuns(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	page := shared.ParsePagination(r, 100, 500)
	filter := reports.JobRunFilter{
		JobType: strings.TrimSpace(r.URL.Query().Get("jobType")),
		Status:  strings.TrimSpace(r.URL.Query().Get("status")),
	}
	filter.Status = strings.ToLower(filter.Status)

	validator := shared.NewValidator()
	if filter.Status != "" {
		validator.Enum("status", filter.Status, []string{"running", "completed", "failed"}, "must be one of: running, completed, failed")
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("startedFrom")); raw != "" {
		startedFrom, err := parseJobRunDate(raw, false)
		if err != nil {
			validator.Add("startedFrom", "must be YYYY-MM-DD or RFC3339 timestamp")
		} else {
			filter.StartedFrom = &startedFrom
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("startedTo")); raw != "" {
		startedTo, err := parseJobRunDate(raw, true)
		if err != nil {
			validator.Add("startedTo", "must be YYYY-MM-DD or RFC3339 timestamp")
		} else {
			filter.StartedTo = &startedTo
		}
	}
	if filter.StartedFrom != nil && filter.StartedTo != nil && filter.StartedTo.Before(*filter.StartedFrom) {
		validator.Add("startedFrom", "must be on or before startedTo")
		validator.Add("startedTo", "must be on or after startedFrom")
	}
	if validator.Reject(w, middleware.GetRequestID(r.Context())) {
		return
	}

	total, err := h.Service.CountJobRuns(r.Context(), user.TenantID, filter)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "job_runs_failed", "failed to count job runs", middleware.GetRequestID(r.Context()))
		return
	}

	runs, err := h.Service.JobRuns(r.Context(), user.TenantID, filter, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "job_runs_failed", "failed to list job runs", middleware.GetRequestID(r.Context()))
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	api.Success(w, runs, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleJobRunDetail(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	runID := strings.TrimSpace(chi.URLParam(r, "runID"))
	if runID == "" {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "runID", Reason: "is required"},
		})
		return
	}

	run, err := h.Service.JobRunByID(r.Context(), user.TenantID, runID)
	if errors.Is(err, pgx.ErrNoRows) {
		api.Fail(w, http.StatusNotFound, "not_found", "job run not found", middleware.GetRequestID(r.Context()))
		return
	}
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "job_run_failed", "failed to load job run", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, run, middleware.GetRequestID(r.Context()))
}

func parseJobRunDate(raw string, endOfDay bool) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed, nil
	}
	parsed, err := shared.ParseDate(raw)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		return parsed.Add(24*time.Hour - time.Nanosecond), nil
	}
	return parsed, nil
}
