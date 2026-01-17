package reportshandler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/leave"
	"hrm/internal/domain/reports"
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
	r.Route("/reports", func(r chi.Router) {
		r.Get("/dashboard/employee", h.handleEmployeeDashboard)
		r.Get("/dashboard/manager", h.handleManagerDashboard)
		r.Get("/dashboard/hr", h.handleHRDashboard)
	})
}

func (h *Handler) handleEmployeeDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	var employeeID string
	_ = h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID)

	var leaveBalance float64
	_ = h.DB.QueryRow(r.Context(), "SELECT COALESCE(SUM(balance),0) FROM leave_balances WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&leaveBalance)

	var payslipCount int
	_ = h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM payslips WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&payslipCount)

	var goalCount int
	_ = h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM goals WHERE tenant_id = $1 AND employee_id = $2", user.TenantID, employeeID).Scan(&goalCount)

	api.Success(w, reports.EmployeeDashboard(leaveBalance, payslipCount, goalCount), middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleManagerDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	var managerEmployeeID string
	_ = h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID)

	var pendingApprovals int
	_ = h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status = $2", user.TenantID, leave.StatusPending).Scan(&pendingApprovals)

	var teamGoals int
	_ = h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM goals WHERE tenant_id = $1 AND manager_id = $2", user.TenantID, managerEmployeeID).Scan(&teamGoals)

	var reviewTasks int
	_ = h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM review_tasks WHERE tenant_id = $1 AND manager_id = $2", user.TenantID, managerEmployeeID).Scan(&reviewTasks)

	api.Success(w, reports.ManagerDashboard(pendingApprovals, teamGoals, reviewTasks), middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleHRDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	var payrollPeriods int
	_ = h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM payroll_periods WHERE tenant_id = $1", user.TenantID).Scan(&payrollPeriods)

	var leavePending int
	_ = h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status = $2", user.TenantID, leave.StatusPending).Scan(&leavePending)

	var reviewCycles int
	_ = h.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM review_cycles WHERE tenant_id = $1", user.TenantID).Scan(&reviewCycles)

	api.Success(w, reports.HRDashboard(payrollPeriods, leavePending, reviewCycles), middleware.GetRequestID(r.Context()))
}
