package reportshandler

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/leave"
	"hrm/internal/domain/reports"
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
	r.Route("/reports", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/employee", h.handleEmployeeDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/manager", h.handleManagerDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/hr", h.handleHRDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/employee/export", h.handleExportEmployeeDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/manager/export", h.handleExportManagerDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/dashboard/hr/export", h.handleExportHRDashboard)
		r.With(middleware.RequirePermission(auth.PermReportsRead, h.Perms)).Get("/jobs", h.handleJobRuns)
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

	var employeeID string
	if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
		slog.Warn("employee lookup failed", "err", err)
	}

	var leaveBalance float64
	if err := h.DB.QueryRow(r.Context(), "SELECT COALESCE(SUM(balance),0) FROM leave_balances WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&leaveBalance); err != nil {
		slog.Warn("leave balance aggregate failed", "err", err)
	}

	var payslipCount int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM payslips WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&payslipCount); err != nil {
		slog.Warn("payslip count failed", "err", err)
	}

	var goalCount int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM goals WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&goalCount); err != nil {
		slog.Warn("goal count failed", "err", err)
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

	var managerEmployeeID string
	if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
		slog.Warn("manager employee lookup failed", "err", err)
	}

	var pendingApprovals int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status IN ($2,$3)", user.TenantID, leave.StatusPending, leave.StatusPendingHR).Scan(&pendingApprovals); err != nil {
		slog.Warn("pending approvals count failed", "err", err)
	}

	var teamGoals int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM goals WHERE tenant_id = $1 AND manager_id = $2", user.TenantID, managerEmployeeID).Scan(&teamGoals); err != nil {
		slog.Warn("team goals count failed", "err", err)
	}

	var reviewTasks int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM review_tasks WHERE tenant_id = $1 AND manager_id = $2", user.TenantID, managerEmployeeID).Scan(&reviewTasks); err != nil {
		slog.Warn("review tasks count failed", "err", err)
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
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM payroll_periods WHERE tenant_id = $1", user.TenantID).Scan(&payrollPeriods); err != nil {
		slog.Warn("payroll period count failed", "err", err)
	}

	var leavePending int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status IN ($2,$3)", user.TenantID, leave.StatusPending, leave.StatusPendingHR).Scan(&leavePending); err != nil {
		slog.Warn("leave pending count failed", "err", err)
	}

	var reviewCycles int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM review_cycles WHERE tenant_id = $1", user.TenantID).Scan(&reviewCycles); err != nil {
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
	var employeeID string
	if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "export_failed", "failed to export dashboard", middleware.GetRequestID(r.Context()))
		return
	}

	var leaveBalance float64
	if err := h.DB.QueryRow(r.Context(), "SELECT COALESCE(SUM(balance),0) FROM leave_balances WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&leaveBalance); err != nil {
		slog.Warn("leave balance aggregate failed", "err", err)
	}
	var payslipCount int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM payslips WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&payslipCount); err != nil {
		slog.Warn("payslip count failed", "err", err)
	}
	var goalCount int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM goals WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&goalCount); err != nil {
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
	var managerEmployeeID string
	if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "export_failed", "failed to export dashboard", middleware.GetRequestID(r.Context()))
		return
	}

	var pendingApprovals int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status IN ($2,$3)", user.TenantID, leave.StatusPending, leave.StatusPendingHR).Scan(&pendingApprovals); err != nil {
		slog.Warn("pending approvals count failed", "err", err)
	}
	var teamGoals int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM goals WHERE tenant_id = $1 AND manager_id = $2", user.TenantID, managerEmployeeID).Scan(&teamGoals); err != nil {
		slog.Warn("team goals count failed", "err", err)
	}
	var reviewTasks int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM review_tasks WHERE tenant_id = $1 AND manager_id = $2", user.TenantID, managerEmployeeID).Scan(&reviewTasks); err != nil {
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

	var payrollPeriods int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM payroll_periods WHERE tenant_id = $1", user.TenantID).Scan(&payrollPeriods); err != nil {
		slog.Warn("payroll period count failed", "err", err)
	}
	var leavePending int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status IN ($2,$3)", user.TenantID, leave.StatusPending, leave.StatusPendingHR).Scan(&leavePending); err != nil {
		slog.Warn("leave pending count failed", "err", err)
	}
	var reviewCycles int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM review_cycles WHERE tenant_id = $1", user.TenantID).Scan(&reviewCycles); err != nil {
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
	jobType := r.URL.Query().Get("jobType")
	query := `
    SELECT id, job_type, status, details_json, started_at, completed_at
    FROM job_runs
    WHERE tenant_id = $1
  `
	args := []any{user.TenantID}
	if jobType != "" {
		query += " AND job_type = $2"
		args = append(args, jobType)
	}
	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	query += fmt.Sprintf(" ORDER BY started_at DESC LIMIT $%d OFFSET $%d", limitPos, offsetPos)
	args = append(args, page.Limit, page.Offset)

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "job_runs_failed", "failed to list job runs", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var runs []map[string]any
	for rows.Next() {
		var id, jobTypeVal, status string
		var details any
		var startedAt, completedAt any
		if err := rows.Scan(&id, &jobTypeVal, &status, &details, &startedAt, &completedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "job_runs_failed", "failed to list job runs", middleware.GetRequestID(r.Context()))
			return
		}
		runs = append(runs, map[string]any{
			"id":          id,
			"jobType":     jobTypeVal,
			"status":      status,
			"details":     details,
			"startedAt":   startedAt,
			"completedAt": completedAt,
		})
	}
	api.Success(w, runs, middleware.GetRequestID(r.Context()))
}
