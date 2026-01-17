package payrollhandler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/payroll"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
)

type Handler struct {
	DB *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{DB: db}
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

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/payroll", func(r chi.Router) {
		r.Get("/schedules", h.handleListSchedules)
		r.Post("/schedules", h.handleCreateSchedule)
		r.Get("/elements", h.handleListElements)
		r.Post("/elements", h.handleCreateElement)
		r.Get("/periods", h.handleListPeriods)
		r.Post("/periods", h.handleCreatePeriod)
		r.Get("/periods/{periodID}/inputs", h.handleListInputs)
		r.Post("/periods/{periodID}/inputs", h.handleCreateInput)
		r.Post("/periods/{periodID}/run", h.handleRunPayroll)
		r.Post("/periods/{periodID}/finalize", h.handleFinalizePayroll)
		r.Get("/payslips", h.handleListPayslips)
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
	if user.RoleName != "HR" {
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
	if user.RoleName != "HR" {
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
	if user.RoleName != "HR" {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload PayrollPeriod
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO payroll_periods (tenant_id, schedule_id, start_date, end_date)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, user.TenantID, payload.ScheduleID, payload.StartDate, payload.EndDate).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_period_create_failed", "failed to create period", middleware.GetRequestID(r.Context()))
		return
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
	if user.RoleName != "HR" {
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
	if user.RoleName != "HR" {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, salary, currency
    FROM employees
    WHERE tenant_id = $1 AND status = 'active'
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to load employees", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var employeeID string
		var salary float64
		var currency string
		if err := rows.Scan(&employeeID, &salary, &currency); err != nil {
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

		_, _ = h.DB.Exec(r.Context(), `
      INSERT INTO payroll_results (tenant_id, period_id, employee_id, gross, deductions, net, currency)
      VALUES ($1,$2,$3,$4,$5,$6,$7)
      ON CONFLICT (period_id, employee_id) DO UPDATE SET gross = EXCLUDED.gross, deductions = EXCLUDED.deductions, net = EXCLUDED.net
    `, user.TenantID, periodID, employeeID, gross, deductions, net, currency)
	}

	api.Success(w, map[string]string{"status": "payroll_ran"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleFinalizePayroll(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != "HR" {
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
    UPDATE payroll_periods SET status = 'finalized', finalized_at = now() WHERE id = $1 AND tenant_id = $2
  `, periodID, user.TenantID)
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

	response := map[string]string{"status": "finalized"}
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
	if employeeID == "" {
		_ = h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID)
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT p.id, p.period_id, p.employee_id, r.gross, r.deductions, r.net, r.currency, p.created_at
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
		var id, periodID, empID, currency string
		var gross, deductions, net float64
		var created time.Time
		if err := rows.Scan(&id, &periodID, &empID, &gross, &deductions, &net, &currency, &created); err != nil {
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
			"createdAt":  created,
		})
	}

	api.Success(w, slips, middleware.GetRequestID(r.Context()))
}
