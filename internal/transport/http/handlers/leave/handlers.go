package leavehandler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/leave"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	DB *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{DB: db}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/leave", func(r chi.Router) {
		r.Get("/types", h.handleListTypes)
		r.Post("/types", h.handleCreateType)
		r.Get("/policies", h.handleListPolicies)
		r.Post("/policies", h.handleCreatePolicy)
		r.Get("/balances", h.handleListBalances)
		r.Post("/balances/adjust", h.handleAdjustBalance)
		r.Get("/requests", h.handleListRequests)
		r.Post("/requests", h.handleCreateRequest)
		r.Post("/requests/{requestID}/approve", h.handleApproveRequest)
		r.Post("/requests/{requestID}/reject", h.handleRejectRequest)
		r.Post("/requests/{requestID}/cancel", h.handleCancelRequest)
		r.Get("/calendar", h.handleCalendar)
		r.Get("/reports/balances", h.handleReportBalances)
		r.Get("/reports/usage", h.handleReportUsage)
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
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListBalances(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID := r.URL.Query().Get("employeeId")
	if employeeID == "" {
		_ = h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID)
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

	api.Success(w, map[string]string{"status": "adjusted"}, middleware.GetRequestID(r.Context()))
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
		_ = h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID)
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		_ = h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID)
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
		_ = h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID)
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

	_, _ = h.DB.Exec(r.Context(), `
    INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
    VALUES ($1,$2,$3,0,$4,0)
    ON CONFLICT (employee_id, leave_type_id) DO UPDATE SET pending = leave_balances.pending + EXCLUDED.pending, updated_at = now()
  `, user.TenantID, payload.EmployeeID, payload.LeaveTypeID, days)

	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
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

	_, _ = h.DB.Exec(r.Context(), `
    UPDATE leave_requests SET status = $1, approved_by = $2, approved_at = now() WHERE id = $3
  `, leave.StatusApproved, user.UserID, requestID)

	_, _ = h.DB.Exec(r.Context(), `
    UPDATE leave_balances
    SET pending = pending - $1, used = used + $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, user.TenantID, employeeID, leaveTypeID)

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

	_, _ = h.DB.Exec(r.Context(), `
    UPDATE leave_requests SET status = $1, approved_by = $2, approved_at = now() WHERE id = $3
  `, leave.StatusRejected, user.UserID, requestID)

	_, _ = h.DB.Exec(r.Context(), `
    UPDATE leave_balances
    SET pending = pending - $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, user.TenantID, employeeID, leaveTypeID)

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

	_, _ = h.DB.Exec(r.Context(), `
    UPDATE leave_requests SET status = $1, cancelled_at = now() WHERE id = $2
  `, leave.StatusCancelled, requestID)

	_, _ = h.DB.Exec(r.Context(), `
    UPDATE leave_balances
    SET pending = pending - $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, user.TenantID, employeeID, leaveTypeID)

	api.Success(w, map[string]string{"status": leave.StatusCancelled}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCalendar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, employee_id, leave_type_id, start_date, end_date, status
    FROM leave_requests
    WHERE tenant_id = $1 AND status = ANY($2)
  `, user.TenantID, []string{leave.StatusPending, leave.StatusApproved})
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

func (h *Handler) handleReportBalances(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
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
