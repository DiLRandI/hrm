package payrollhandler

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jung-kurt/gofpdf"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/audit"
	"hrm/internal/domain/core"
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

type payrollPeriodPayload struct {
	ScheduleID string `json:"scheduleId"`
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/payroll", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/schedules", h.handleListSchedules)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/schedules", h.handleCreateSchedule)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/elements", h.handleListElements)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/elements", h.handleCreateElement)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/periods", h.handleListPeriods)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/periods", h.handleCreatePeriod)
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/periods/{periodID}/inputs", h.handleListInputs)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/periods/{periodID}/inputs", h.handleCreateInput)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/periods/{periodID}/inputs/import", h.handleImportInputs)
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
	_ = audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.schedule.create", "pay_schedule", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload)
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

	_ = audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.element.create", "pay_element", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload)
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

	_ = audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.period.create", "payroll_period", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload)
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

		gross, deductions, net := payroll.ComputePayroll(salary, inputs)

		var warnings []string
		if bankAccount == "" {
			warnings = append(warnings, "missing_bank_account")
		}
		if net < 0 {
			warnings = append(warnings, "negative_net")
		}
		var previousNet float64
		_ = h.DB.QueryRow(r.Context(), `
      SELECT net
      FROM payroll_results
      WHERE tenant_id = $1 AND employee_id = $2
      ORDER BY created_at DESC
      LIMIT 1
    `, user.TenantID, employeeID).Scan(&previousNet)
		if previousNet > 0 {
			diff := net - previousNet
			if diff < 0 {
				diff = -diff
			}
			if diff/previousNet > 0.5 {
				warnings = append(warnings, "net_variance")
			}
		}
		warningsJSON, _ := json.Marshal(warnings)

		_, _ = h.DB.Exec(r.Context(), `
      INSERT INTO payroll_results (tenant_id, period_id, employee_id, gross, deductions, net, currency, warnings_json)
      VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
      ON CONFLICT (period_id, employee_id)
      DO UPDATE SET gross = EXCLUDED.gross, deductions = EXCLUDED.deductions, net = EXCLUDED.net, warnings_json = EXCLUDED.warnings_json
    `, user.TenantID, periodID, employeeID, gross, deductions, net, currency, warningsJSON)
	}

	_, _ = h.DB.Exec(r.Context(), `
    UPDATE payroll_periods SET status = $1 WHERE id = $2 AND tenant_id = $3
  `, payroll.PeriodStatusReviewed, periodID, user.TenantID)
	_ = audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.run", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil)

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
	idempotencyKey := r.Header.Get("Idempotency-Key")
	requestHash := middleware.RequestHash([]byte(periodID))
	if idempotencyKey != "" {
		stored, found, _ := middleware.CheckIdempotency(r.Context(), h.DB, user.TenantID, user.UserID, "payroll.finalize", idempotencyKey, requestHash)
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

	_, _ = h.DB.Exec(r.Context(), `
    INSERT INTO payslips (tenant_id, period_id, employee_id)
    SELECT tenant_id, period_id, employee_id
    FROM payroll_results
    WHERE period_id = $1
    ON CONFLICT DO NOTHING
  `, periodID)

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
				continue
			}
			fileURL, err := h.generatePayslipPDF(r.Context(), user.TenantID, periodID, employeeID, payslipID)
			if err == nil && fileURL != "" {
				_, _ = h.DB.Exec(r.Context(), "UPDATE payslips SET file_url = $1 WHERE id = $2", fileURL, payslipID)
			}
			var employeeUserID string
			_ = h.DB.QueryRow(r.Context(), "SELECT user_id FROM employees WHERE tenant_id = $1 AND id = $2", user.TenantID, employeeID).Scan(&employeeUserID)
			if employeeUserID != "" {
				_ = notify.Create(r.Context(), user.TenantID, employeeUserID, "payslip_published", "Payslip published", "A new payslip is available for download.")
			}
		}
	}

	_ = audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.finalize", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil)

	response := map[string]string{"status": payroll.PeriodStatusFinalized}
	payload, _ := json.Marshal(response)
	if idempotencyKey != "" {
		_ = middleware.SaveIdempotency(r.Context(), h.DB, user.TenantID, user.UserID, "payroll.finalize", idempotencyKey, requestHash, payload)
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
		_ = h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID)
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
	reader := csv.NewReader(r.Body)
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
				_ = h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND email = $2", user.TenantID, email).Scan(&employeeID)
			}
		}
		elementID := get(row, "element_id")
		units, _ := strconv.ParseFloat(get(row, "units"), 64)
		rate, _ := strconv.ParseFloat(get(row, "rate"), 64)
		amount, _ := strconv.ParseFloat(get(row, "amount"), 64)
		source := get(row, "source")
		if amount == 0 && units > 0 {
			amount = units * rate
		}
		if employeeID == "" || elementID == "" {
			continue
		}
		_, _ = h.DB.Exec(r.Context(), `
      INSERT INTO payroll_inputs (tenant_id, period_id, employee_id, element_id, units, rate, amount, source)
      VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `, user.TenantID, periodID, employeeID, elementID, units, rate, amount, source)
		inserted++
	}

	_ = audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.inputs.import", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"count": inserted})
	api.Success(w, map[string]any{"imported": inserted}, middleware.GetRequestID(r.Context()))
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
	_ = json.NewDecoder(r.Body).Decode(&payload)

	_, err := h.DB.Exec(r.Context(), `
    UPDATE payroll_periods SET status = $1 WHERE id = $2 AND tenant_id = $3
  `, payroll.PeriodStatusDraft, periodID, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_reopen_failed", "failed to reopen payroll", middleware.GetRequestID(r.Context()))
		return
	}
	_, _ = h.DB.Exec(r.Context(), "DELETE FROM payroll_results WHERE tenant_id = $1 AND period_id = $2", user.TenantID, periodID)
	_, _ = h.DB.Exec(r.Context(), "DELETE FROM payslips WHERE tenant_id = $1 AND period_id = $2", user.TenantID, periodID)

	_ = audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payroll.reopen", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload)
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
	_ = writer.Write([]string{"employee_id", "first_name", "last_name", "gross", "deductions", "net", "currency"})
	for rows.Next() {
		var id, first, last, currency string
		var gross, deductions, net float64
		if err := rows.Scan(&id, &first, &last, &gross, &deductions, &net, &currency); err != nil {
			api.Fail(w, http.StatusInternalServerError, "export_failed", "failed to export register", middleware.GetRequestID(r.Context()))
			return
		}
		_ = writer.Write([]string{id, first, last, fmt.Sprintf("%.2f", gross), fmt.Sprintf("%.2f", deductions), fmt.Sprintf("%.2f", net), currency})
	}
	writer.Flush()
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
	_ = writer.Write([]string{"account", "debit", "credit"})
	_ = writer.Write([]string{"Payroll Expense", fmt.Sprintf("%.2f", gross), ""})
	_ = writer.Write([]string{"Payroll Deductions", "", fmt.Sprintf("%.2f", deductions)})
	_ = writer.Write([]string{"Payroll Cash", "", fmt.Sprintf("%.2f", net)})
	writer.Flush()
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
		_ = h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID)
		if selfEmployeeID == "" || employeeID != selfEmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if fileURL == "" {
		var periodID string
		_ = h.DB.QueryRow(r.Context(), "SELECT period_id FROM payslips WHERE tenant_id = $1 AND id = $2", user.TenantID, payslipID).Scan(&periodID)
		fileURL, _ = h.generatePayslipPDF(r.Context(), user.TenantID, periodID, employeeID, payslipID)
		if fileURL != "" {
			_, _ = h.DB.Exec(r.Context(), "UPDATE payslips SET file_url = $1 WHERE id = $2", fileURL, payslipID)
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
	_, _ = h.DB.Exec(r.Context(), "UPDATE payslips SET file_url = $1 WHERE id = $2", fileURL, payslipID)

	_ = audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "payslip.regenerate", "payslip", payslipID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil)
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
