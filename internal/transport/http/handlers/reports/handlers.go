package reportshandler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/leave"
	"hrm/internal/domain/reports"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
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
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status = $2", user.TenantID, leave.StatusPending).Scan(&pendingApprovals); err != nil {
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
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status = $2", user.TenantID, leave.StatusPending).Scan(&leavePending); err != nil {
		slog.Warn("leave pending count failed", "err", err)
	}

	var reviewCycles int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM review_cycles WHERE tenant_id = $1", user.TenantID).Scan(&reviewCycles); err != nil {
		slog.Warn("review cycles count failed", "err", err)
	}

	api.Success(w, reports.HRDashboard(payrollPeriods, leavePending, reviewCycles), middleware.GetRequestID(r.Context()))
}
