package payrollhandler

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jung-kurt/gofpdf"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
	"hrm/internal/domain/leave"
	"hrm/internal/domain/notifications"
	"hrm/internal/domain/payroll"
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

type PaySchedule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Frequency string    `json:"frequency"`
	PayDay    int       `json:"payDay"`
	CreatedAt time.Time `json:"createdAt"`
}

type PayGroup struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ScheduleID string `json:"scheduleId"`
	Currency   string `json:"currency"`
}

type PayElement struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	ElementType string  `json:"elementType"`
	CalcType    string  `json:"calcType"`
	Amount      float64 `json:"amount"`
	Taxable     bool    `json:"taxable"`
}

type PayrollPeriod struct {
	ID         string    `json:"id"`
	ScheduleID string    `json:"scheduleId"`
	StartDate  time.Time `json:"startDate"`
	EndDate    time.Time `json:"endDate"`
	Status     string    `json:"status"`
}

type PayrollAdjustment struct {
	ID          string    `json:"id"`
	EmployeeID  string    `json:"employeeId"`
	Description string    `json:"description"`
	Amount      float64   `json:"amount"`
	CreatedAt   time.Time `json:"createdAt"`
}

type payrollPeriodPayload struct {
	ScheduleID string `json:"scheduleId"`
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/payroll", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/schedules", h.handleListSchedules)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/schedules", h.handleCreateSchedule)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/groups", h.handleListGroups)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/groups", h.handleCreateGroup)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/elements", h.handleListElements)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/elements", h.handleCreateElement)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/periods", h.handleListPeriods)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/periods", h.handleCreatePeriod)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/periods/{periodID}/inputs", h.handleListInputs)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/periods/{periodID}/inputs", h.handleCreateInput)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/periods/{periodID}/inputs/import", h.handleImportInputs)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/periods/{periodID}/adjustments", h.handleListAdjustments)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/periods/{periodID}/adjustments", h.handleCreateAdjustment)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/periods/{periodID}/summary", h.handlePeriodSummary)
		r.With(middleware.RequirePermission(auth.PermPayrollRun, h.Perms)).Post("/periods/{periodID}/run", h.handleRunPayroll)
		r.With(middleware.RequirePermission(auth.PermPayrollFinalize, h.Perms)).Post("/periods/{periodID}/finalize", h.handleFinalizePayroll)
		r.With(middleware.RequirePermission(auth.PermPayrollFinalize, h.Perms)).Post("/periods/{periodID}/reopen", h.handleReopenPeriod)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/periods/{periodID}/export/register", h.handleExportRegister)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/periods/{periodID}/export/journal", h.handleExportJournal)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/payslips", h.handleListPayslips)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/payslips/{payslipID}/download", h.handleDownloadPayslip)
		r.With(middleware.RequirePermission(auth.PermPayrollFinalize, h.Perms)).Post("/payslips/{payslipID}/regenerate", h.handleRegeneratePayslip)
	})
}

func (h *Handler) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	rows, err := h.DB.Query(r.Context(), `
    SELECT id, name, frequency, pay_day, created_at
    FROM pay_schedules
    WHERE tenant_id = $1
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_schedules_failed", "failed to list schedules", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var schedules []PaySchedule
	for rows.Next() {
		var s PaySchedule
		if err := rows.Scan(&s.ID, &s.Name, &s.Frequency, &s.PayDay, &s.CreatedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_schedules_failed", "failed to list schedules", middleware.GetRequestID(r.Context()))
			return
		}
		schedules = append(schedules, s)
	}
	api.Success(w, schedules, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload PaySchedule
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO pay_schedules (tenant_id, name, frequency, pay_day)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, user.TenantID, payload.Name, payload.Frequency, payload.PayDay).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_schedule_create_failed", "failed to create schedule", middleware.GetRequestID(r.Context()))
		return
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.schedule.create", "pay_schedule", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		log.Printf("audit payroll.schedule.create failed: %v", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, name, COALESCE(schedule_id, ''), COALESCE(currency, 'USD')
    FROM pay_groups
    WHERE tenant_id = $1
    ORDER BY name
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_groups_failed", "failed to list pay groups", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var groups []PayGroup
	for rows.Next() {
		var g PayGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.ScheduleID, &g.Currency); err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_groups_failed", "failed to list pay groups", middleware.GetRequestID(r.Context()))
			return
		}
		groups = append(groups, g)
	}
	api.Success(w, groups, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload PayGroup
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO pay_groups (tenant_id, name, schedule_id, currency)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, user.TenantID, payload.Name, nullIfEmpty(payload.ScheduleID), payload.Currency).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_group_create_failed", "failed to create pay group", middleware.GetRequestID(r.Context()))
		return
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.group.create", "pay_group", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		log.Printf("audit payroll.group.create failed: %v", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListElements(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	rows, err := h.DB.Query(r.Context(), `
    SELECT id, name, element_type, calc_type, amount, taxable
    FROM pay_elements
    WHERE tenant_id = $1
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_elements_failed", "failed to list elements", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var elements []PayElement
	for rows.Next() {
		var e PayElement
		if err := rows.Scan(&e.ID, &e.Name, &e.ElementType, &e.CalcType, &e.Amount, &e.Taxable); err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_elements_failed", "failed to list elements", middleware.GetRequestID(r.Context()))
			return
		}
		elements = append(elements, e)
	}
	api.Success(w, elements, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateElement(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload PayElement
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO pay_elements (tenant_id, name, element_type, calc_type, amount, taxable)
    VALUES ($1,$2,$3,$4,$5,$6)
    RETURNING id
  `, user.TenantID, payload.Name, payload.ElementType, payload.CalcType, payload.Amount, payload.Taxable).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_element_create_failed", "failed to create element", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.element.create", "pay_element", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		log.Printf("audit payroll.element.create failed: %v", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListPeriods(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	rows, err := h.DB.Query(r.Context(), `
    SELECT id, schedule_id, start_date, end_date, status
    FROM payroll_periods
    WHERE tenant_id = $1
    ORDER BY start_date DESC
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_periods_failed", "failed to list periods", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var periods []PayrollPeriod
	for rows.Next() {
		var p PayrollPeriod
		if err := rows.Scan(&p.ID, &p.ScheduleID, &p.StartDate, &p.EndDate, &p.Status); err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_periods_failed", "failed to list periods", middleware.GetRequestID(r.Context()))
			return
		}
		periods = append(periods, p)
	}
	api.Success(w, periods, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreatePeriod(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload payrollPeriodPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.ScheduleID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "schedule id required", middleware.GetRequestID(r.Context()))
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

	var id string
	err = h.DB.QueryRow(r.Context(), `
    INSERT INTO payroll_periods (tenant_id, schedule_id, start_date, end_date)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, user.TenantID, payload.ScheduleID, startDate, endDate).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_period_create_failed", "failed to create period", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.period.create", "payroll_period", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		log.Printf("audit payroll.period.create failed: %v", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

type payrollInput struct {
	EmployeeID string  `json:"employeeId"`
	ElementID  string  `json:"elementId"`
	Units      float64 `json:"units"`
	Rate       float64 `json:"rate"`
	Amount     float64 `json:"amount"`
	Source     string  `json:"source"`
}

func (h *Handler) handleListInputs(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	rows, err := h.DB.Query(r.Context(), `
    SELECT employee_id, element_id, units, rate, amount, source
    FROM payroll_inputs
    WHERE tenant_id = $1 AND period_id = $2
  `, user.TenantID, periodID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_inputs_failed", "failed to list inputs", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var inputs []payrollInput
	for rows.Next() {
		var input payrollInput
		if err := rows.Scan(&input.EmployeeID, &input.ElementID, &input.Units, &input.Rate, &input.Amount, &input.Source); err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_inputs_failed", "failed to list inputs", middleware.GetRequestID(r.Context()))
			return
		}
		inputs = append(inputs, input)
	}
	api.Success(w, inputs, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateInput(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	var payload payrollInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	if payload.Amount == 0 && payload.Units > 0 {
		payload.Amount = payload.Units * payload.Rate
	}
	if payload.Source == "" {
		payload.Source = payroll.InputSourceManual
	}

	_, err := h.DB.Exec(r.Context(), `
    INSERT INTO payroll_inputs (tenant_id, period_id, employee_id, element_id, units, rate, amount, source)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
  `, user.TenantID, periodID, payload.EmployeeID, payload.ElementID, payload.Units, payload.Rate, payload.Amount, payload.Source)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_input_failed", "failed to create input", middleware.GetRequestID(r.Context()))
		return
	}

	api.Created(w, map[string]string{"status": "input_added"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleRunPayroll(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	var periodStatus string
	var periodStart, periodEnd time.Time
	err := h.DB.QueryRow(r.Context(), `
    SELECT status, start_date, end_date
    FROM payroll_periods
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, periodID).Scan(&periodStatus, &periodStart, &periodEnd)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "payroll period not found", middleware.GetRequestID(r.Context()))
		return
	}
	if periodStatus == payroll.PeriodStatusFinalized {
		api.Fail(w, http.StatusBadRequest, "invalid_state", "payroll period already finalized", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, salary, currency, COALESCE(bank_account, '')
    FROM employees
    WHERE tenant_id = $1 AND status = $2
  `, user.TenantID, core.EmployeeStatusActive)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to load employees", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var employeeID string
		var salary float64
		var currency string
		var bankAccount string
		if err := rows.Scan(&employeeID, &salary, &currency, &bankAccount); err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to load employees", middleware.GetRequestID(r.Context()))
			return
		}

		inputRows, err := h.DB.Query(r.Context(), `
      SELECT pe.element_type, pi.amount
      FROM payroll_inputs pi
      JOIN pay_elements pe ON pi.element_id = pe.id
      WHERE pi.period_id = $1 AND pi.employee_id = $2
    `, periodID, employeeID)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to load inputs", middleware.GetRequestID(r.Context()))
			return
		}

		var inputs []payroll.InputLine
		for inputRows.Next() {
			var etype string
			var amount float64
			if err := inputRows.Scan(&etype, &amount); err != nil {
				inputRows.Close()
				api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to process inputs", middleware.GetRequestID(r.Context()))
				return
			}
			inputs = append(inputs, payroll.InputLine{Type: etype, Amount: amount})
		}
		inputRows.Close()

		adjRows, err := h.DB.Query(r.Context(), `
      SELECT amount
      FROM payroll_adjustments
      WHERE tenant_id = $1 AND period_id = $2 AND employee_id = $3
    `, user.TenantID, periodID, employeeID)
		if err == nil {
			for adjRows.Next() {
				var amount float64
				if err := adjRows.Scan(&amount); err != nil {
					continue
				}
				if amount >= 0 {
					inputs = append(inputs, payroll.InputLine{Type: payroll.ElementTypeEarning, Amount: amount})
				} else {
					inputs = append(inputs, payroll.InputLine{Type: payroll.ElementTypeDeduction, Amount: -amount})
				}
			}
			adjRows.Close()
		}

		var unpaidDays float64
		leaveRows, err := h.DB.Query(r.Context(), `
      SELECT lr.start_date, lr.end_date, lr.start_half, lr.end_half
      FROM leave_requests lr
      JOIN leave_types lt ON lr.leave_type_id = lt.id
      WHERE lr.tenant_id = $1
        AND lr.employee_id = $2
        AND lr.status = $3
        AND lt.is_paid = false
        AND lr.start_date <= $4
        AND lr.end_date >= $5
    `, user.TenantID, employeeID, leave.StatusApproved, periodEnd, periodStart)
		if err == nil {
			for leaveRows.Next() {
				var startDate, endDate time.Time
				var startHalf, endHalf bool
				if err := leaveRows.Scan(&startDate, &endDate, &startHalf, &endHalf); err != nil {
					continue
				}
				overlapStart := startDate
				if periodStart.After(overlapStart) {
					overlapStart = periodStart
				}
				overlapEnd := endDate
				if periodEnd.Before(overlapEnd) {
					overlapEnd = periodEnd
				}
				days, err := leave.CalculateDays(overlapStart, overlapEnd)
				if err != nil {
					continue
				}
				if startHalf && overlapStart.Equal(startDate) {
					days -= 0.5
				}
				if endHalf && overlapEnd.Equal(endDate) {
					days -= 0.5
				}
				if days > 0 {
					unpaidDays += days
				}
			}
			leaveRows.Close()
		}

		if unpaidDays > 0 && salary > 0 {
			periodDays, err := leave.CalculateDays(periodStart, periodEnd)
			if err == nil && periodDays > 0 {
				deduction := (salary / periodDays) * unpaidDays
				if deduction > 0 {
					inputs = append(inputs, payroll.InputLine{Type: payroll.ElementTypeDeduction, Amount: deduction})
				}
			}
		}

		gross, deductions, net := payroll.ComputePayroll(salary, inputs)

		var warnings []string
		if bankAccount == "" {
			warnings = append(warnings, payroll.WarningMissingBank)
		}
		if net < 0 {
			warnings = append(warnings, payroll.WarningNegativeNet)
		}
		var previousNet float64
		if err := h.DB.QueryRow(r.Context(), `
      SELECT net
      FROM payroll_results
      WHERE tenant_id = $1 AND employee_id = $2
      ORDER BY created_at DESC
      LIMIT 1
    `, user.TenantID, employeeID).Scan(&previousNet); err != nil {
			log.Printf("previous net lookup failed: %v", err)
		}
		if previousNet > 0 {
			diff := net - previousNet
			if diff < 0 {
				diff = -diff
			}
			if diff/previousNet > 0.5 {
				warnings = append(warnings, payroll.WarningNetVariance)
			}
		}
		warningsJSON, err := json.Marshal(warnings)
		if err != nil {
			log.Printf("warnings marshal failed: %v", err)
			warningsJSON = []byte("[]")
		}

		if _, err := h.DB.Exec(r.Context(), `
      INSERT INTO payroll_results (tenant_id, period_id, employee_id, gross, deductions, net, currency, warnings_json)
      VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
      ON CONFLICT (period_id, employee_id)
      DO UPDATE SET gross = EXCLUDED.gross, deductions = EXCLUDED.deductions, net = EXCLUDED.net, warnings_json = EXCLUDED.warnings_json
    `, user.TenantID, periodID, employeeID, gross, deductions, net, currency, warningsJSON); err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to persist payroll results", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE payroll_periods SET status = $1 WHERE id = $2 AND tenant_id = $3
  `, payroll.PeriodStatusReviewed, periodID, user.TenantID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to update payroll period", middleware.GetRequestID(r.Context()))
		return
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.run", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		log.Printf("audit payroll.run failed: %v", err)
	}

	api.Success(w, map[string]string{"status": payroll.PeriodStatusReviewed}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleFinalizePayroll(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	var currentStatus string
	if err := h.DB.QueryRow(r.Context(), `
    SELECT status
    FROM payroll_periods
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, periodID).Scan(&currentStatus); err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "payroll period not found", middleware.GetRequestID(r.Context()))
		return
	}
	if currentStatus != payroll.PeriodStatusReviewed {
		api.Fail(w, http.StatusBadRequest, "invalid_state", "payroll period must be reviewed before finalize", middleware.GetRequestID(r.Context()))
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	requestHash := middleware.RequestHash([]byte(periodID))
	if idempotencyKey != "" {
		stored, found, err := middleware.CheckIdempotency(r.Context(), h.DB, user.TenantID, user.UserID, "payroll.finalize", idempotencyKey, requestHash)
		if err != nil {
			log.Printf("idempotency check failed: %v", err)
		}
		if found {
			api.Success(w, json.RawMessage(stored), middleware.GetRequestID(r.Context()))
			return
		}
	}

	_, err := h.DB.Exec(r.Context(), `
    UPDATE payroll_periods SET status = $1, finalized_at = now() WHERE id = $2 AND tenant_id = $3
  `, payroll.PeriodStatusFinalized, periodID, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_finalize_failed", "failed to finalize payroll", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), `
    INSERT INTO payslips (tenant_id, period_id, employee_id)
    SELECT tenant_id, period_id, employee_id
    FROM payroll_results
    WHERE period_id = $1
    ON CONFLICT DO NOTHING
  `, periodID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_finalize_failed", "failed to generate payslips", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, employee_id
    FROM payslips
    WHERE tenant_id = $1 AND period_id = $2
  `, user.TenantID, periodID)
	if err == nil {
		defer rows.Close()
		notify := notifications.New(h.DB)
		for rows.Next() {
			var payslipID, employeeID string
			if err := rows.Scan(&payslipID, &employeeID); err != nil {
				log.Printf("payslip scan failed: %v", err)
				continue
			}
			fileURL, err := h.generatePayslipPDF(r.Context(), user.TenantID, periodID, employeeID, payslipID)
			if err != nil {
				log.Printf("payslip pdf generation failed: %v", err)
			} else if fileURL != "" {
				if _, err := h.DB.Exec(r.Context(), "UPDATE payslips SET file_url = $1 WHERE id = $2", fileURL, payslipID); err != nil {
					log.Printf("payslip file url update failed: %v", err)
				}
			}
			var employeeUserID string
			if err := h.DB.QueryRow(r.Context(), "SELECT user_id FROM employees WHERE tenant_id = $1 AND id = $2", user.TenantID, employeeID).Scan(&employeeUserID); err != nil {
				log.Printf("payslip employee user lookup failed: %v", err)
			}
			if employeeUserID != "" {
				if err := notify.Create(r.Context(), user.TenantID, employeeUserID, notifications.TypePayslipPublished, "Payslip published", "A new payslip is available for download."); err != nil {
					log.Printf("payslip notification failed: %v", err)
				}
			}
		}
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.finalize", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		log.Printf("audit payroll.finalize failed: %v", err)
	}

	response := map[string]string{"status": payroll.PeriodStatusFinalized}
	payload, err := json.Marshal(response)
	if err != nil {
		log.Printf("finalize response marshal failed: %v", err)
	}
	if idempotencyKey != "" {
		if err := middleware.SaveIdempotency(r.Context(), h.DB, user.TenantID, user.UserID, "payroll.finalize", idempotencyKey, requestHash, payload); err != nil {
			log.Printf("idempotency save failed: %v", err)
		}
	}

	api.Success(w, response, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListPayslips(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID := r.URL.Query().Get("employeeId")
	if user.RoleName != auth.RoleHR {
		employeeID = ""
	}
	if employeeID == "" {
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			log.Printf("payslip list employee lookup failed: %v", err)
		}
	}
	if employeeID == "" {
		api.Success(w, []map[string]any{}, middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT p.id, p.period_id, p.employee_id, r.gross, r.deductions, r.net, r.currency, p.file_url, p.created_at
    FROM payslips p
    JOIN payroll_results r ON p.period_id = r.period_id AND p.employee_id = r.employee_id
    WHERE p.tenant_id = $1 AND p.employee_id = $2
    ORDER BY p.created_at DESC
  `, user.TenantID, employeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payslip_list_failed", "failed to list payslips", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var slips []map[string]any
	for rows.Next() {
		var id, periodID, empID, currency, fileURL string
		var gross, deductions, net float64
		var created time.Time
		if err := rows.Scan(&id, &periodID, &empID, &gross, &deductions, &net, &currency, &fileURL, &created); err != nil {
			api.Fail(w, http.StatusInternalServerError, "payslip_list_failed", "failed to list payslips", middleware.GetRequestID(r.Context()))
			return
		}
		slips = append(slips, map[string]any{
			"id":         id,
			"periodId":   periodID,
			"employeeId": empID,
			"gross":      gross,
			"deductions": deductions,
			"net":        net,
			"currency":   currency,
			"fileUrl":    fileURL,
			"createdAt":  created,
		})
	}

	api.Success(w, slips, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleImportInputs(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "unable to read csv payload", middleware.GetRequestID(r.Context()))
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	requestHash := middleware.RequestHash(body)
	if idempotencyKey != "" {
		stored, found, err := middleware.CheckIdempotency(r.Context(), h.DB, user.TenantID, user.UserID, "payroll.inputs.import", idempotencyKey, requestHash)
		if err != nil {
			log.Printf("idempotency check failed: %v", err)
		}
		if found {
			api.Success(w, json.RawMessage(stored), middleware.GetRequestID(r.Context()))
			return
		}
	}

	reader := csv.NewReader(bytes.NewReader(body))
	headers, err := reader.Read()
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid csv payload", middleware.GetRequestID(r.Context()))
		return
	}

	index := map[string]int{}
	for i, h := range headers {
		index[strings.ToLower(strings.TrimSpace(h))] = i
	}

	get := func(row []string, key string) string {
		if idx, ok := index[key]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	inserted := 0
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid csv payload", middleware.GetRequestID(r.Context()))
			return
		}
		employeeID := get(row, "employee_id")
		if employeeID == "" {
			if email := get(row, "employee_email"); email != "" {
				if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND email = $2", user.TenantID, email).Scan(&employeeID); err != nil {
					log.Printf("import input employee lookup failed for %s: %v", email, err)
				}
			}
		}
		elementID := get(row, "element_id")
		units, err := strconv.ParseFloat(get(row, "units"), 64)
		if err != nil {
			log.Printf("import input units parse failed: %v", err)
			units = 0
		}
		rate, err := strconv.ParseFloat(get(row, "rate"), 64)
		if err != nil {
			log.Printf("import input rate parse failed: %v", err)
			rate = 0
		}
		amount, err := strconv.ParseFloat(get(row, "amount"), 64)
		if err != nil {
			log.Printf("import input amount parse failed: %v", err)
			amount = 0
		}
		source := get(row, "source")
		if source == "" {
			source = payroll.InputSourceImport
		}
		if amount == 0 && units > 0 {
			amount = units * rate
		}
		if employeeID == "" || elementID == "" {
			continue
		}
		if _, err := h.DB.Exec(r.Context(), `
      INSERT INTO payroll_inputs (tenant_id, period_id, employee_id, element_id, units, rate, amount, source)
      VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `, user.TenantID, periodID, employeeID, elementID, units, rate, amount, source); err != nil {
			log.Printf("import input insert failed: %v", err)
			continue
		}
		inserted++
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.inputs.import", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"count": inserted}); err != nil {
		log.Printf("audit payroll.inputs.import failed: %v", err)
	}
	response := map[string]any{"imported": inserted}
	if idempotencyKey != "" {
		encoded, err := json.Marshal(response)
		if err != nil {
			log.Printf("idempotency response marshal failed: %v", err)
		} else if err := middleware.SaveIdempotency(r.Context(), h.DB, user.TenantID, user.UserID, "payroll.inputs.import", idempotencyKey, requestHash, encoded); err != nil {
			log.Printf("idempotency save failed: %v", err)
		}
	}
	api.Success(w, response, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListAdjustments(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	query := `
    SELECT id, employee_id, description, amount, created_at
    FROM payroll_adjustments
    WHERE tenant_id = $1 AND period_id = $2
  `
	args := []any{user.TenantID, periodID}

	if user.RoleName == auth.RoleEmployee {
		var employeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			log.Printf("adjustments list employee lookup failed: %v", err)
		}
		query += " AND employee_id = $3"
		args = append(args, employeeID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "adjustments_list_failed", "failed to list adjustments", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var adjustments []PayrollAdjustment
	for rows.Next() {
		var adj PayrollAdjustment
		if err := rows.Scan(&adj.ID, &adj.EmployeeID, &adj.Description, &adj.Amount, &adj.CreatedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "adjustments_list_failed", "failed to list adjustments", middleware.GetRequestID(r.Context()))
			return
		}
		adjustments = append(adjustments, adj)
	}
	api.Success(w, adjustments, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateAdjustment(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	var payload struct {
		EmployeeID  string  `json:"employeeId"`
		Description string  `json:"description"`
		Amount      float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.EmployeeID == "" || payload.Description == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee id and description required", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO payroll_adjustments (tenant_id, period_id, employee_id, description, amount)
    VALUES ($1,$2,$3,$4,$5)
    RETURNING id
  `, user.TenantID, periodID, payload.EmployeeID, payload.Description, payload.Amount).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "adjustment_create_failed", "failed to create adjustment", middleware.GetRequestID(r.Context()))
		return
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.adjustment.create", "payroll_adjustment", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		log.Printf("audit payroll.adjustment.create failed: %v", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handlePeriodSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	var gross, deductions, net float64
	var count int
	if err := h.DB.QueryRow(r.Context(), `
    SELECT COALESCE(SUM(gross),0), COALESCE(SUM(deductions),0), COALESCE(SUM(net),0), COUNT(1)
    FROM payroll_results
    WHERE tenant_id = $1 AND period_id = $2
  `, user.TenantID, periodID).Scan(&gross, &deductions, &net, &count); err != nil {
		log.Printf("period summary totals query failed: %v", err)
	}

	warningCounts := map[string]int{}
	rows, err := h.DB.Query(r.Context(), `
    SELECT warnings_json
    FROM payroll_results
    WHERE tenant_id = $1 AND period_id = $2
  `, user.TenantID, periodID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				log.Printf("period summary warnings scan failed: %v", err)
				continue
			}
			var warnings []string
			if err := json.Unmarshal(raw, &warnings); err != nil {
				log.Printf("period summary warnings unmarshal failed: %v", err)
				continue
			}
			for _, wKey := range warnings {
				warningCounts[wKey]++
			}
		}
	}

	api.Success(w, map[string]any{
		"totalGross":      gross,
		"totalDeductions": deductions,
		"totalNet":        net,
		"employeeCount":   count,
		"warnings":        warningCounts,
	}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleReopenPeriod(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("reopen payload decode failed: %v", err)
	}
	if strings.TrimSpace(payload.Reason) == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "reopen reason required", middleware.GetRequestID(r.Context()))
		return
	}

	var currentStatus string
	if err := h.DB.QueryRow(r.Context(), `
    SELECT status
    FROM payroll_periods
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, periodID).Scan(&currentStatus); err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "payroll period not found", middleware.GetRequestID(r.Context()))
		return
	}
	if currentStatus != payroll.PeriodStatusFinalized {
		api.Fail(w, http.StatusBadRequest, "invalid_state", "only finalized periods can be reopened", middleware.GetRequestID(r.Context()))
		return
	}

	_, err := h.DB.Exec(r.Context(), `
    UPDATE payroll_periods SET status = $1 WHERE id = $2 AND tenant_id = $3
  `, payroll.PeriodStatusDraft, periodID, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_reopen_failed", "failed to reopen payroll", middleware.GetRequestID(r.Context()))
		return
	}
	if _, err := h.DB.Exec(r.Context(), "DELETE FROM payroll_results WHERE tenant_id = $1 AND period_id = $2", user.TenantID, periodID); err != nil {
		log.Printf("payroll results delete failed: %v", err)
	}
	if _, err := h.DB.Exec(r.Context(), "DELETE FROM payslips WHERE tenant_id = $1 AND period_id = $2", user.TenantID, periodID); err != nil {
		log.Printf("payslips delete failed: %v", err)
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.reopen", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		log.Printf("audit payroll.reopen failed: %v", err)
	}
	api.Success(w, map[string]string{"status": payroll.PeriodStatusDraft}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleExportRegister(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	rows, err := h.DB.Query(r.Context(), `
    SELECT e.id, e.first_name, e.last_name, r.gross, r.deductions, r.net, r.currency
    FROM payroll_results r
    JOIN employees e ON r.employee_id = e.id
    WHERE r.tenant_id = $1 AND r.period_id = $2
    ORDER BY e.last_name, e.first_name
  `, user.TenantID, periodID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "export_failed", "failed to export register", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=payroll-register.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"employee_id", "first_name", "last_name", "gross", "deductions", "net", "currency"}); err != nil {
		log.Printf("export register header write failed: %v", err)
	}
	for rows.Next() {
		var id, first, last, currency string
		var gross, deductions, net float64
		if err := rows.Scan(&id, &first, &last, &gross, &deductions, &net, &currency); err != nil {
			api.Fail(w, http.StatusInternalServerError, "export_failed", "failed to export register", middleware.GetRequestID(r.Context()))
			return
		}
		if err := writer.Write([]string{id, first, last, fmt.Sprintf("%.2f", gross), fmt.Sprintf("%.2f", deductions), fmt.Sprintf("%.2f", net), currency}); err != nil {
			log.Printf("export register row write failed: %v", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Printf("export register flush failed: %v", err)
	}
}

func (h *Handler) handleExportJournal(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	var gross, deductions, net float64
	err := h.DB.QueryRow(r.Context(), `
    SELECT COALESCE(SUM(gross),0), COALESCE(SUM(deductions),0), COALESCE(SUM(net),0)
    FROM payroll_results
    WHERE tenant_id = $1 AND period_id = $2
  `, user.TenantID, periodID).Scan(&gross, &deductions, &net)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "export_failed", "failed to export journal", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=payroll-journal.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"account", "debit", "credit"}); err != nil {
		log.Printf("export journal header write failed: %v", err)
	}
	if err := writer.Write([]string{"Payroll Expense", fmt.Sprintf("%.2f", gross), ""}); err != nil {
		log.Printf("export journal expense row write failed: %v", err)
	}
	if err := writer.Write([]string{"Payroll Deductions", "", fmt.Sprintf("%.2f", deductions)}); err != nil {
		log.Printf("export journal deductions row write failed: %v", err)
	}
	if err := writer.Write([]string{"Payroll Cash", "", fmt.Sprintf("%.2f", net)}); err != nil {
		log.Printf("export journal cash row write failed: %v", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Printf("export journal flush failed: %v", err)
	}
}

func (h *Handler) handleDownloadPayslip(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	payslipID := chi.URLParam(r, "payslipID")
	var employeeID, fileURL string
	err := h.DB.QueryRow(r.Context(), `
    SELECT employee_id, COALESCE(file_url, '')
    FROM payslips
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, payslipID).Scan(&employeeID, &fileURL)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "payslip not found", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName != auth.RoleHR {
		var selfEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
			log.Printf("payslip download self employee lookup failed: %v", err)
		}
		if selfEmployeeID == "" || employeeID != selfEmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if fileURL == "" {
		var periodID string
		if err := h.DB.QueryRow(r.Context(), "SELECT period_id FROM payslips WHERE tenant_id = $1 AND id = $2", user.TenantID, payslipID).Scan(&periodID); err != nil {
			log.Printf("payslip period lookup failed: %v", err)
		}
		fileURL, err = h.generatePayslipPDF(r.Context(), user.TenantID, periodID, employeeID, payslipID)
		if err != nil {
			log.Printf("payslip pdf generation failed: %v", err)
		}
		if fileURL != "" {
			if _, err := h.DB.Exec(r.Context(), "UPDATE payslips SET file_url = $1 WHERE id = $2", fileURL, payslipID); err != nil {
				log.Printf("payslip file url update failed: %v", err)
			}
		}
	}

	if fileURL == "" {
		api.Fail(w, http.StatusInternalServerError, "payslip_missing", "payslip not available", middleware.GetRequestID(r.Context()))
		return
	}

	http.ServeFile(w, r, fileURL)
}

func (h *Handler) handleRegeneratePayslip(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	payslipID := chi.URLParam(r, "payslipID")
	var employeeID, periodID string
	err := h.DB.QueryRow(r.Context(), `
    SELECT employee_id, period_id
    FROM payslips
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, payslipID).Scan(&employeeID, &periodID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "payslip not found", middleware.GetRequestID(r.Context()))
		return
	}

	fileURL, err := h.generatePayslipPDF(r.Context(), user.TenantID, periodID, employeeID, payslipID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payslip_generate_failed", "failed to regenerate payslip", middleware.GetRequestID(r.Context()))
		return
	}
	if _, err := h.DB.Exec(r.Context(), "UPDATE payslips SET file_url = $1 WHERE id = $2", fileURL, payslipID); err != nil {
		log.Printf("payslip regenerate update failed: %v", err)
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payslip.regenerate", "payslip", payslipID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		log.Printf("audit payslip.regenerate failed: %v", err)
	}
	api.Success(w, map[string]string{"status": "regenerated"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) generatePayslipPDF(ctx context.Context, tenantID, periodID, employeeID, payslipID string) (string, error) {
	var firstName, lastName, email, currency string
	var gross, deductions, net float64
	var startDate, endDate time.Time
	err := h.DB.QueryRow(ctx, `
    SELECT e.first_name, e.last_name, e.email,
           r.gross, r.deductions, r.net, r.currency,
           p.start_date, p.end_date
    FROM payroll_results r
    JOIN employees e ON r.employee_id = e.id
    JOIN payroll_periods p ON r.period_id = p.id
    WHERE r.tenant_id = $1 AND r.period_id = $2 AND r.employee_id = $3
  `, tenantID, periodID, employeeID).Scan(&firstName, &lastName, &email, &gross, &deductions, &net, &currency, &startDate, &endDate)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll("storage/payslips", 0o755); err != nil {
		return "", err
	}
	filePath := filepath.Join("storage/payslips", payslipID+".pdf")

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(40, 10, "Payslip")
	pdf.Ln(12)
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(0, 8, fmt.Sprintf("Employee: %s %s", firstName, lastName))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Email: %s", email))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Period: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")))
	pdf.Ln(10)
	pdf.Cell(0, 8, fmt.Sprintf("Gross: %.2f %s", gross, currency))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Deductions: %.2f %s", deductions, currency))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Net: %.2f %s", net, currency))

	if err := pdf.OutputFileAndClose(filePath); err != nil {
		return "", err
	}
	return filePath, nil
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
