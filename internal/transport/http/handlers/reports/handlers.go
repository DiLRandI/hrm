package reportshandler

import (
	"log"
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
		log.Printf("employee lookup failed: %v", err)
	}

	var leaveBalance float64
	if err := h.DB.QueryRow(r.Context(), "SELECT COALESCE(SUM(balance),0) FROM leave_balances WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&leaveBalance); err != nil {
		log.Printf("leave balance aggregate failed: %v", err)
	}

	var payslipCount int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM payslips WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&payslipCount); err != nil {
		log.Printf("payslip count failed: %v", err)
	}

	var goalCount int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM goals WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&goalCount); err != nil {
		log.Printf("goal count failed: %v", err)
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
		log.Printf("manager employee lookup failed: %v", err)
	}

	var pendingApprovals int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status = $2", user.TenantID, leave.StatusPending).Scan(&pendingApprovals); err != nil {
		log.Printf("pending approvals count failed: %v", err)
	}

	var teamGoals int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM goals WHERE tenant_id = $1 AND manager_id = $2", user.TenantID, managerEmployeeID).Scan(&teamGoals); err != nil {
		log.Printf("team goals count failed: %v", err)
	}

	var reviewTasks int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM review_tasks WHERE tenant_id = $1 AND manager_id = $2", user.TenantID, managerEmployeeID).Scan(&reviewTasks); err != nil {
		log.Printf("review tasks count failed: %v", err)
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
		log.Printf("payroll period count failed: %v", err)
	}

	var leavePending int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status = $2", user.TenantID, leave.StatusPending).Scan(&leavePending); err != nil {
		log.Printf("leave pending count failed: %v", err)
	}

	var reviewCycles int
	if err := h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM review_cycles WHERE tenant_id = $1", user.TenantID).Scan(&reviewCycles); err != nil {
		log.Printf("review cycles count failed: %v", err)
	}

	api.Success(w, reports.HRDashboard(payrollPeriods, leavePending, reviewCycles), middleware.GetRequestID(r.Context()))
}
