package leavehandler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/domain/leave"
	"hrm/internal/domain/notifications"
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
		r.With(middleware.RequirePermission(auth.PermLeaveWrite, h.Perms)).Post("/requests", h.handleCreateRequest)
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
	rows, err := h.DB.Query(r.Context(), `
    SELECT id, name, code, is_paid, requires_doc, created_at
    FROM leave_types
    WHERE tenant_id = $1
    ORDER BY name
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_types_failed", "failed to list leave types", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var types []leave.LeaveType
	for rows.Next() {
		var t leave.LeaveType
		if err := rows.Scan(&t.ID, &t.Name, &t.Code, &t.IsPaid, &t.RequiresDoc, &t.CreatedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "leave_types_failed", "failed to list leave types", middleware.GetRequestID(r.Context()))
			return
		}
		types = append(types, t)
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

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO leave_types (tenant_id, name, code, is_paid, requires_doc)
    VALUES ($1,$2,$3,$4,$5)
    RETURNING id
  `, user.TenantID, payload.Name, payload.Code, payload.IsPaid, payload.RequiresDoc).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_type_create_failed", "failed to create leave type", middleware.GetRequestID(r.Context()))
		return
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "leave.type.create", "leave_type", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, leave_type_id, accrual_rate, accrual_period, entitlement, carry_over_limit, allow_negative
    FROM leave_policies
    WHERE tenant_id = $1
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_policies_failed", "failed to list leave policies", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var policies []leave.LeavePolicy
	for rows.Next() {
		var p leave.LeavePolicy
		if err := rows.Scan(&p.ID, &p.LeaveTypeID, &p.AccrualRate, &p.AccrualPeriod, &p.Entitlement, &p.CarryOver, &p.AllowNegative); err != nil {
			api.Fail(w, http.StatusInternalServerError, "leave_policies_failed", "failed to list leave policies", middleware.GetRequestID(r.Context()))
			return
		}
		policies = append(policies, p)
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

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO leave_policies (tenant_id, leave_type_id, accrual_rate, accrual_period, entitlement, carry_over_limit, allow_negative)
    VALUES ($1,$2,$3,$4,$5,$6,$7)
    RETURNING id
  `, user.TenantID, payload.LeaveTypeID, payload.AccrualRate, payload.AccrualPeriod, payload.Entitlement, payload.CarryOver, payload.AllowNegative).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_policy_create_failed", "failed to create leave policy", middleware.GetRequestID(r.Context()))
		return
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "leave.policy.create", "leave_policy", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, date, name, region
    FROM holidays
    WHERE tenant_id = $1
    ORDER BY date
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "holiday_list_failed", "failed to list holidays", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, name, region string
		var date time.Time
		if err := rows.Scan(&id, &date, &name, &region); err != nil {
			api.Fail(w, http.StatusInternalServerError, "holiday_list_failed", "failed to list holidays", middleware.GetRequestID(r.Context()))
			return
		}
		out = append(out, map[string]any{
			"id":     id,
			"date":   date,
			"name":   name,
			"region": region,
		})
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

	var id string
	err = h.DB.QueryRow(r.Context(), `
    INSERT INTO holidays (tenant_id, date, name, region)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, user.TenantID, holidayDate, payload.Name, payload.Region).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "holiday_create_failed", "failed to create holiday", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "leave.holiday.create", "holiday", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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
	_, err := h.DB.Exec(r.Context(), "DELETE FROM holidays WHERE tenant_id = $1 AND id = $2", user.TenantID, holidayID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "holiday_delete_failed", "failed to delete holiday", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "leave.holiday.delete", "holiday", holidayID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
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
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
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
			var allowed int
			if err := h.DB.QueryRow(r.Context(), `
        SELECT COUNT(1)
        FROM employees
        WHERE tenant_id = $1 AND id = $2 AND manager_id = $3
      `, user.TenantID, employeeID, selfEmployeeID).Scan(&allowed); err != nil {
				slog.Warn("leave balances manager scope check failed", "err", err)
			}
			if allowed == 0 {
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

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, employee_id, leave_type_id, balance, pending, used, updated_at
    FROM leave_balances
    WHERE tenant_id = $1 AND employee_id = $2
  `, user.TenantID, employeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_balances_failed", "failed to list balances", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var balances []map[string]any
	for rows.Next() {
		var id, employee, leaveType string
		var balance, pending, used float64
		var updatedAt time.Time
		if err := rows.Scan(&id, &employee, &leaveType, &balance, &pending, &used, &updatedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "leave_balances_failed", "failed to list balances", middleware.GetRequestID(r.Context()))
			return
		}
		balances = append(balances, map[string]any{
			"id":          id,
			"employeeId":  employee,
			"leaveTypeId": leaveType,
			"balance":     balance,
			"pending":     pending,
			"used":        used,
			"updatedAt":   updatedAt,
		})
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

	_, err := h.DB.Exec(r.Context(), `
    INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
    VALUES ($1,$2,$3,$4,0,0)
    ON CONFLICT (employee_id, leave_type_id) DO UPDATE SET balance = leave_balances.balance + EXCLUDED.balance, updated_at = now()
  `, user.TenantID, payload.EmployeeID, payload.LeaveTypeID, payload.Amount)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_adjust_failed", "failed to adjust balance", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), `
    INSERT INTO leave_balance_adjustments (tenant_id, employee_id, leave_type_id, amount, reason, created_by)
    VALUES ($1,$2,$3,$4,$5,$6)
  `, user.TenantID, payload.EmployeeID, payload.LeaveTypeID, payload.Amount, payload.Reason, user.UserID); err != nil {
		slog.Warn("leave balance adjustment insert failed", "err", err)
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "leave.balance.adjust", "leave_balance", payload.EmployeeID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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

	summary, err := leave.ApplyAccruals(r.Context(), h.DB, user.TenantID, time.Now())
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "accrual_failed", "failed to run accruals", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "leave.accrual.run", "leave_policy", "", middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, summary); err != nil {
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

	query := `
    SELECT id, employee_id, leave_type_id, start_date, end_date, days, reason, status, created_at
    FROM leave_requests
    WHERE tenant_id = $1
  `
	args := []any{user.TenantID}

	if user.RoleName == auth.RoleEmployee {
		var employeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			slog.Warn("leave requests employee lookup failed", "err", err)
		}
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("leave requests manager lookup failed", "err", err)
		}
		query += " AND employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $2)"
		args = append(args, managerEmployeeID)
	}

	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_requests_failed", "failed to list requests", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var requests []leave.LeaveRequest
	for rows.Next() {
		var req leave.LeaveRequest
		if err := rows.Scan(&req.ID, &req.EmployeeID, &req.LeaveTypeID, &req.StartDate, &req.EndDate, &req.Days, &req.Reason, &req.Status, &req.CreatedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "leave_requests_failed", "failed to list requests", middleware.GetRequestID(r.Context()))
			return
		}
		requests = append(requests, req)
	}
	api.Success(w, requests, middleware.GetRequestID(r.Context()))
}

type leaveRequestPayload struct {
	EmployeeID  string `json:"employeeId"`
	LeaveTypeID string `json:"leaveTypeId"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Reason      string `json:"reason"`
}

func (h *Handler) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload leaveRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.LeaveTypeID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "leave type required", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName != auth.RoleHR {
		var selfEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
			slog.Warn("leave request self employee lookup failed", "err", err)
		}
		payload.EmployeeID = selfEmployeeID
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

	days, err := leave.CalculateDays(startDate, endDate)
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_dates", "invalid date range", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	err = h.DB.QueryRow(r.Context(), `
    INSERT INTO leave_requests (tenant_id, employee_id, leave_type_id, start_date, end_date, days, reason)
    VALUES ($1,$2,$3,$4,$5,$6,$7)
    RETURNING id
  `, user.TenantID, payload.EmployeeID, payload.LeaveTypeID, startDate, endDate, days, payload.Reason).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "leave_request_failed", "failed to create request", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), `
    INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
    VALUES ($1,$2,$3,0,$4,0)
    ON CONFLICT (employee_id, leave_type_id) DO UPDATE SET pending = leave_balances.pending + EXCLUDED.pending, updated_at = now()
  `, user.TenantID, payload.EmployeeID, payload.LeaveTypeID, days); err != nil {
		slog.Warn("leave request balance pending update failed", "err", err)
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "leave.request.create", "leave_request", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit leave.request.create failed", "err", err)
	}
	var managerUserID string
	if err := h.DB.QueryRow(r.Context(), `
    SELECT m.user_id
    FROM employees e
    JOIN employees m ON e.manager_id = m.id
    WHERE e.tenant_id = $1 AND e.id = $2
  `, user.TenantID, payload.EmployeeID).Scan(&managerUserID); err != nil {
		slog.Warn("leave request manager lookup failed", "err", err)
	}
	if managerUserID != "" {
		if err := notifications.New(h.DB).Create(r.Context(), user.TenantID, managerUserID, notifications.TypeLeaveSubmitted, "Leave request submitted", "A leave request is awaiting approval."); err != nil {
			slog.Warn("leave submitted notification failed", "err", err)
		}
	}

	api.Created(w, map[string]string{"id": id, "status": leave.StatusPending}, middleware.GetRequestID(r.Context()))
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
	var employeeID, leaveTypeID string
	var days float64
	err := h.DB.QueryRow(r.Context(), `
    SELECT employee_id, leave_type_id, days
    FROM leave_requests
    WHERE id = $1 AND tenant_id = $2
  `, requestID, user.TenantID).Scan(&employeeID, &leaveTypeID, &days)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "leave request not found", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE leave_requests SET status = $1, approved_by = $2, approved_at = now() WHERE id = $3
  `, leave.StatusApproved, user.UserID, requestID); err != nil {
		slog.Warn("leave request approve update failed", "err", err)
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE leave_balances
    SET pending = pending - $1, used = used + $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, user.TenantID, employeeID, leaveTypeID); err != nil {
		slog.Warn("leave balance approve update failed", "err", err)
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "leave.request.approve", "leave_request", requestID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"employeeId": employeeID}); err != nil {
		slog.Warn("audit leave.request.approve failed", "err", err)
	}
	var employeeUserID, leaveTypeName string
	if err := h.DB.QueryRow(r.Context(), `
    SELECT e.user_id, lt.name
    FROM employees e
    JOIN leave_types lt ON lt.id = $2
    WHERE e.tenant_id = $1 AND e.id = $3
  `, user.TenantID, leaveTypeID, employeeID).Scan(&employeeUserID, &leaveTypeName); err != nil {
		slog.Warn("leave approve employee lookup failed", "err", err)
	}
	if employeeUserID != "" {
		title := "Leave approved"
		body := fmt.Sprintf("Your %s leave request was approved.", leaveTypeName)
		if err := notifications.New(h.DB).Create(r.Context(), user.TenantID, employeeUserID, notifications.TypeLeaveApproved, title, body); err != nil {
			slog.Warn("leave approved notification failed", "err", err)
		}
	}

	api.Success(w, map[string]string{"status": leave.StatusApproved}, middleware.GetRequestID(r.Context()))
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
	var employeeID, leaveTypeID string
	var days float64
	err := h.DB.QueryRow(r.Context(), `
    SELECT employee_id, leave_type_id, days
    FROM leave_requests
    WHERE id = $1 AND tenant_id = $2
  `, requestID, user.TenantID).Scan(&employeeID, &leaveTypeID, &days)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "leave request not found", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE leave_requests SET status = $1, approved_by = $2, approved_at = now() WHERE id = $3
  `, leave.StatusRejected, user.UserID, requestID); err != nil {
		slog.Warn("leave request reject update failed", "err", err)
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE leave_balances
    SET pending = pending - $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, user.TenantID, employeeID, leaveTypeID); err != nil {
		slog.Warn("leave balance reject update failed", "err", err)
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "leave.request.reject", "leave_request", requestID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"employeeId": employeeID}); err != nil {
		slog.Warn("audit leave.request.reject failed", "err", err)
	}
	var employeeUserID, leaveTypeName string
	if err := h.DB.QueryRow(r.Context(), `
    SELECT e.user_id, lt.name
    FROM employees e
    JOIN leave_types lt ON lt.id = $2
    WHERE e.tenant_id = $1 AND e.id = $3
  `, user.TenantID, leaveTypeID, employeeID).Scan(&employeeUserID, &leaveTypeName); err != nil {
		slog.Warn("leave reject employee lookup failed", "err", err)
	}
	if employeeUserID != "" {
		title := "Leave rejected"
		body := fmt.Sprintf("Your %s leave request was rejected.", leaveTypeName)
		if err := notifications.New(h.DB).Create(r.Context(), user.TenantID, employeeUserID, notifications.TypeLeaveRejected, title, body); err != nil {
			slog.Warn("leave rejected notification failed", "err", err)
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
	var employeeID, leaveTypeID string
	var days float64
	var status string
	err := h.DB.QueryRow(r.Context(), `
    SELECT employee_id, leave_type_id, days, status
    FROM leave_requests
    WHERE id = $1 AND tenant_id = $2
  `, requestID, user.TenantID).Scan(&employeeID, &leaveTypeID, &days, &status)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "leave request not found", middleware.GetRequestID(r.Context()))
		return
	}

	if status == leave.StatusApproved {
		api.Fail(w, http.StatusBadRequest, "invalid_state", "approved requests require HR cancellation", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE leave_requests SET status = $1, cancelled_at = now() WHERE id = $2
  `, leave.StatusCancelled, requestID); err != nil {
		slog.Warn("leave request cancel update failed", "err", err)
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE leave_balances
    SET pending = pending - $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, user.TenantID, employeeID, leaveTypeID); err != nil {
		slog.Warn("leave balance cancel update failed", "err", err)
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "leave.request.cancel", "leave_request", requestID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"employeeId": employeeID}); err != nil {
		slog.Warn("audit leave.request.cancel failed", "err", err)
	}
	var employeeUserID, leaveTypeName string
	if err := h.DB.QueryRow(r.Context(), `
    SELECT e.user_id, lt.name
    FROM employees e
    JOIN leave_types lt ON lt.id = $2
    WHERE e.tenant_id = $1 AND e.id = $3
  `, user.TenantID, leaveTypeID, employeeID).Scan(&employeeUserID, &leaveTypeName); err != nil {
		slog.Warn("leave cancel employee lookup failed", "err", err)
	}
	if employeeUserID != "" {
		title := "Leave cancelled"
		body := fmt.Sprintf("Your %s leave request was cancelled.", leaveTypeName)
		if err := notifications.New(h.DB).Create(r.Context(), user.TenantID, employeeUserID, notifications.TypeLeaveCancelled, title, body); err != nil {
			slog.Warn("leave cancelled notification failed", "err", err)
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

	query := `
    SELECT id, employee_id, leave_type_id, start_date, end_date, status
    FROM leave_requests
    WHERE tenant_id = $1 AND status = ANY($2)
  `
	args := []any{user.TenantID, []string{leave.StatusPending, leave.StatusApproved}}

	if user.RoleName == auth.RoleEmployee {
		var employeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			slog.Warn("calendar employee lookup failed", "err", err)
		}
		query += " AND employee_id = $3"
		args = append(args, employeeID)
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("calendar manager lookup failed", "err", err)
		}
		query += " AND employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $3)"
		args = append(args, managerEmployeeID)
	}

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "calendar_failed", "failed to load calendar", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var events []map[string]any
	for rows.Next() {
		var id, employeeID, leaveTypeID, status string
		var startDate, endDate time.Time
		if err := rows.Scan(&id, &employeeID, &leaveTypeID, &startDate, &endDate, &status); err != nil {
			api.Fail(w, http.StatusInternalServerError, "calendar_failed", "failed to load calendar", middleware.GetRequestID(r.Context()))
			return
		}
		events = append(events, map[string]any{
			"id":          id,
			"employeeId":  employeeID,
			"leaveTypeId": leaveTypeID,
			"start":       startDate,
			"end":         endDate,
			"status":      status,
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

	query := `
    SELECT lr.id, lr.employee_id, lt.name, lr.start_date, lr.end_date, lr.status
    FROM leave_requests lr
    JOIN leave_types lt ON lr.leave_type_id = lt.id
    WHERE lr.tenant_id = $1 AND lr.status = ANY($2)
  `
	args := []any{user.TenantID, []string{leave.StatusPending, leave.StatusApproved}}
	if user.RoleName == auth.RoleEmployee {
		var employeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			slog.Warn("calendar export employee lookup failed", "err", err)
		}
		query += " AND lr.employee_id = $3"
		args = append(args, employeeID)
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("calendar export manager lookup failed", "err", err)
		}
		query += " AND lr.employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $3)"
		args = append(args, managerEmployeeID)
	}
	query += " ORDER BY lr.start_date"

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "calendar_failed", "failed to load calendar", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	if format == "ics" {
		w.Header().Set("Content-Type", "text/calendar")
		w.Header().Set("Content-Disposition", "attachment; filename=leave-calendar.ics")
		var builder strings.Builder
		builder.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//PulseHR//Leave Calendar//EN\r\n")
		for rows.Next() {
			var id, employeeID, leaveTypeName, status string
			var startDate, endDate time.Time
			if err := rows.Scan(&id, &employeeID, &leaveTypeName, &startDate, &endDate, &status); err != nil {
				api.Fail(w, http.StatusInternalServerError, "calendar_failed", "failed to export calendar", middleware.GetRequestID(r.Context()))
				return
			}
			builder.WriteString("BEGIN:VEVENT\r\n")
			builder.WriteString(fmt.Sprintf("UID:%s\r\n", id))
			builder.WriteString(fmt.Sprintf("DTSTART:%s\r\n", startDate.Format("20060102")))
			builder.WriteString(fmt.Sprintf("DTEND:%s\r\n", endDate.AddDate(0, 0, 1).Format("20060102")))
			builder.WriteString(fmt.Sprintf("SUMMARY:%s (%s)\r\n", leaveTypeName, status))
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
	for rows.Next() {
		var id, employeeID, leaveTypeName, status string
		var startDate, endDate time.Time
		if err := rows.Scan(&id, &employeeID, &leaveTypeName, &startDate, &endDate, &status); err != nil {
			api.Fail(w, http.StatusInternalServerError, "calendar_failed", "failed to export calendar", middleware.GetRequestID(r.Context()))
			return
		}
		if err := writer.Write([]string{id, employeeID, leaveTypeName, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), status}); err != nil {
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

	rows, err := h.DB.Query(r.Context(), `
    SELECT employee_id, leave_type_id, balance, pending, used
    FROM leave_balances
    WHERE tenant_id = $1
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "report_failed", "failed to load report", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var report []map[string]any
	for rows.Next() {
		var employeeID, leaveTypeID string
		var balance, pending, used float64
		if err := rows.Scan(&employeeID, &leaveTypeID, &balance, &pending, &used); err != nil {
			api.Fail(w, http.StatusInternalServerError, "report_failed", "failed to load report", middleware.GetRequestID(r.Context()))
			return
		}
		report = append(report, map[string]any{
			"employeeId":  employeeID,
			"leaveTypeId": leaveTypeID,
			"balance":     balance,
			"pending":     pending,
			"used":        used,
		})
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

	rows, err := h.DB.Query(r.Context(), `
    SELECT leave_type_id, SUM(days)
    FROM leave_requests
    WHERE tenant_id = $1 AND status = $2
    GROUP BY leave_type_id
  `, user.TenantID, leave.StatusApproved)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "report_failed", "failed to load report", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var report []map[string]any
	for rows.Next() {
		var leaveTypeID string
		var total float64
		if err := rows.Scan(&leaveTypeID, &total); err != nil {
			api.Fail(w, http.StatusInternalServerError, "report_failed", "failed to load report", middleware.GetRequestID(r.Context()))
			return
		}
		report = append(report, map[string]any{
			"leaveTypeId": leaveTypeID,
			"totalDays":   total,
		})
	}
	api.Success(w, report, middleware.GetRequestID(r.Context()))
}
