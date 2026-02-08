package payrollhandler

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
	"hrm/internal/domain/leave"
	"hrm/internal/domain/notifications"
	"hrm/internal/domain/payroll"
	cryptoutil "hrm/internal/platform/crypto"
	"hrm/internal/platform/jobs"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	Service *payroll.Service
	Perms   middleware.PermissionStore
	Idem    *middleware.IdempotencyStore
	Crypto  *cryptoutil.Service
	Notify  *notifications.Service
	Jobs    *jobs.Service
	Audit   *audit.Service
}

func NewHandler(service *payroll.Service, perms middleware.PermissionStore, idem *middleware.IdempotencyStore, crypto *cryptoutil.Service, notify *notifications.Service, jobsSvc *jobs.Service, auditSvc *audit.Service) *Handler {
	return &Handler{Service: service, Perms: perms, Idem: idem, Crypto: crypto, Notify: notify, Jobs: jobsSvc, Audit: auditSvc}
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
		r.With(middleware.RequirePermission(auth.PermPayrollRead, h.Perms)).Get("/journal-templates", h.handleListJournalTemplates)
		r.With(middleware.RequirePermission(auth.PermPayrollWrite, h.Perms)).Post("/journal-templates", h.handleCreateJournalTemplate)
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
	schedules, err := h.Service.ListSchedules(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_schedules_failed", "failed to list schedules", middleware.GetRequestID(r.Context()))
		return
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

	var payload payroll.Schedule
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	id, err := h.Service.CreateSchedule(r.Context(), user.TenantID, payload.Name, payload.Frequency, payload.PayDay)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_schedule_create_failed", "failed to create schedule", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payroll.schedule.create", "pay_schedule", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit payroll.schedule.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	groups, err := h.Service.ListGroups(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_groups_failed", "failed to list pay groups", middleware.GetRequestID(r.Context()))
		return
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

	var payload payroll.Group
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	id, err := h.Service.CreateGroup(r.Context(), user.TenantID, payload.Name, payload.ScheduleID, payload.Currency)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_group_create_failed", "failed to create pay group", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payroll.group.create", "pay_group", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit payroll.group.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListElements(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	elements, err := h.Service.ListElements(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_elements_failed", "failed to list elements", middleware.GetRequestID(r.Context()))
		return
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

	var payload payroll.Element
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	id, err := h.Service.CreateElement(r.Context(), user.TenantID, payload)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_element_create_failed", "failed to create element", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payroll.element.create", "pay_element", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit payroll.element.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListJournalTemplates(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	templates, err := h.Service.ListJournalTemplates(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "journal_template_list_failed", "failed to list journal templates", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, templates, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateJournalTemplate(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload payroll.JournalTemplate
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.Name == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "name required", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.Config == nil {
		payload.Config = map[string]any{}
	}
	id, err := h.Service.CreateJournalTemplate(r.Context(), user.TenantID, payload.Name, payload.Config)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "journal_template_create_failed", "failed to create journal template", middleware.GetRequestID(r.Context()))
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
	page := shared.ParsePagination(r, 100, 500)
	total, err := h.Service.CountPeriods(r.Context(), user.TenantID)
	if err != nil {
		slog.Warn("payroll period count failed", "err", err)
	}
	periods, err := h.Service.ListPeriods(r.Context(), user.TenantID, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_periods_failed", "failed to list periods", middleware.GetRequestID(r.Context()))
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
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
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "must be valid JSON"},
		})
		return
	}

	validator := shared.NewValidator()
	validator.Required("scheduleId", payload.ScheduleID, "is required")
	startDate, startValid := validator.Date("startDate", payload.StartDate)
	endDate, endValid := validator.Date("endDate", payload.EndDate)
	if startValid && endValid {
		validator.DateOrder("startDate", startDate, "endDate", endDate)
	}
	if validator.Reject(w, middleware.GetRequestID(r.Context())) {
		return
	}

	id, err := h.Service.CreatePeriod(r.Context(), user.TenantID, strings.TrimSpace(payload.ScheduleID), startDate, endDate)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_period_create_failed", "failed to create period", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payroll.period.create", "payroll_period", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit payroll.period.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListInputs(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	page := shared.ParsePagination(r, 100, 500)
	total, err := h.Service.CountInputs(r.Context(), user.TenantID, periodID)
	if err != nil {
		slog.Warn("payroll inputs count failed", "err", err)
	}
	inputs, err := h.Service.ListInputs(r.Context(), user.TenantID, periodID, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_inputs_failed", "failed to list inputs", middleware.GetRequestID(r.Context()))
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
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
	var payload payroll.Input
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "must be valid JSON"},
		})
		return
	}

	payload.EmployeeID = strings.TrimSpace(payload.EmployeeID)
	payload.ElementID = strings.TrimSpace(payload.ElementID)
	if payload.Amount == 0 && payload.Units > 0 {
		payload.Amount = payload.Units * payload.Rate
	}
	if payload.Source == "" {
		payload.Source = payroll.InputSourceManual
	}
	payload.Source = strings.ToLower(strings.TrimSpace(payload.Source))

	validator := shared.NewValidator()
	validator.Required("employeeId", payload.EmployeeID, "is required")
	validator.Required("elementId", payload.ElementID, "is required")
	validator.Enum("source", payload.Source, []string{payroll.InputSourceManual, payroll.InputSourceImport}, "must be one of: manual, import")
	if payload.Units < 0 {
		validator.Add("units", "must be greater than or equal to 0")
	}
	if payload.Rate < 0 {
		validator.Add("rate", "must be greater than or equal to 0")
	}
	if payload.Amount <= 0 {
		validator.Add("amount", "must be greater than 0")
	}
	if validator.Reject(w, middleware.GetRequestID(r.Context())) {
		return
	}

	if err := h.Service.CreateInput(r.Context(), user.TenantID, periodID, payload); err != nil {
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
	periodDetails, err := h.Service.GetPeriodDetails(r.Context(), user.TenantID, periodID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "payroll period not found", middleware.GetRequestID(r.Context()))
		return
	}
	if periodDetails.Status == payroll.PeriodStatusFinalized {
		api.Fail(w, http.StatusBadRequest, "invalid_state", "payroll period already finalized", middleware.GetRequestID(r.Context()))
		return
	}

	jobID := ""
	if h.Jobs != nil {
		if id, err := h.Service.CreateJobRun(r.Context(), user.TenantID, "payroll_run"); err != nil {
			slog.Warn("payroll job run insert failed", "err", err)
		} else {
			jobID = id
		}
	}

	employees, err := h.Service.ListActiveEmployeesForRun(r.Context(), user.TenantID, core.EmployeeStatusActive)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to load employees", middleware.GetRequestID(r.Context()))
		return
	}
	processed := 0
	for _, employee := range employees {
		employeeID := employee.EmployeeID
		if employee.GroupScheduleID != "" && employee.GroupScheduleID != periodDetails.ScheduleID {
			continue
		}

		salary := 0.0
		if employee.SalaryPlain != nil {
			salary = *employee.SalaryPlain
		}
		if h.Crypto != nil && h.Crypto.Configured() && len(employee.SalaryEnc) > 0 {
			if decrypted, err := h.Crypto.DecryptString(employee.SalaryEnc); err == nil {
				if parsed, err := strconv.ParseFloat(decrypted, 64); err == nil {
					salary = parsed
				}
			}
		}
		bankAccount := employee.BankPlain
		if h.Crypto != nil && h.Crypto.Configured() && len(employee.BankEnc) > 0 {
			if decrypted, err := h.Crypto.DecryptString(employee.BankEnc); err == nil {
				bankAccount = decrypted
			}
		}

		inputs, err := h.Service.ListInputLines(r.Context(), periodID, employeeID)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to load inputs", middleware.GetRequestID(r.Context()))
			return
		}

		adjustments, err := h.Service.ListAdjustmentAmounts(r.Context(), user.TenantID, periodID, employeeID, periodDetails.StartDate, periodDetails.EndDate)
		if err == nil {
			for _, amount := range adjustments {
				if amount >= 0 {
					inputs = append(inputs, payroll.InputLine{Type: payroll.ElementTypeEarning, Amount: amount})
				} else {
					inputs = append(inputs, payroll.InputLine{Type: payroll.ElementTypeDeduction, Amount: -amount})
				}
			}
		}

		var unpaidDays float64
		leaveWindows, err := h.Service.ListUnpaidLeaves(r.Context(), user.TenantID, employeeID, periodDetails.StartDate, periodDetails.EndDate, leave.StatusApproved)
		if err == nil {
			for _, leaveWindow := range leaveWindows {
				overlapStart := leaveWindow.StartDate
				if periodDetails.StartDate.After(overlapStart) {
					overlapStart = periodDetails.StartDate
				}
				overlapEnd := leaveWindow.EndDate
				if periodDetails.EndDate.Before(overlapEnd) {
					overlapEnd = periodDetails.EndDate
				}
				days, err := leave.CalculateDays(overlapStart, overlapEnd)
				if err != nil {
					continue
				}
				if leaveWindow.StartHalf && overlapStart.Equal(leaveWindow.StartDate) {
					days -= 0.5
				}
				if leaveWindow.EndHalf && overlapEnd.Equal(leaveWindow.EndDate) {
					days -= 0.5
				}
				if days > 0 {
					unpaidDays += days
				}
			}
		}

		if unpaidDays > 0 && salary > 0 {
			periodDays, err := leave.CalculateDays(periodDetails.StartDate, periodDetails.EndDate)
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
		previousNet, err := h.Service.LatestNet(r.Context(), user.TenantID, employeeID)
		if err != nil {
			slog.Warn("previous net lookup failed", "err", err)
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
			slog.Warn("warnings marshal failed", "err", err)
			warningsJSON = []byte("[]")
		}

		if err := h.Service.UpsertPayrollResult(r.Context(), user.TenantID, periodID, employeeID, gross, deductions, net, employee.Currency, warningsJSON); err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to persist payroll results", middleware.GetRequestID(r.Context()))
			return
		}
		processed++
	}

	if err := h.Service.UpdatePeriodStatus(r.Context(), user.TenantID, periodID, payroll.PeriodStatusReviewed); err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_run_failed", "failed to update payroll period", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payroll.run", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		slog.Warn("audit payroll.run failed", "err", err)
	}

	if jobID != "" {
		details := map[string]any{
			"processed": processed,
			"periodId":  periodID,
		}
		if err := h.Service.UpdateJobRun(r.Context(), jobID, "completed", details); err != nil {
			slog.Warn("payroll job run update failed", "err", err)
		}
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
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "Idempotency-Key", Reason: "header is required for payroll finalization"},
		})
		return
	}

	requestHash := middleware.RequestHash([]byte(periodID))
	stored, found, err := h.Idem.Check(r.Context(), user.TenantID, user.UserID, "payroll.finalize", idempotencyKey, requestHash)
	if errors.Is(err, middleware.ErrIdempotencyConflict) {
		api.Fail(w, http.StatusConflict, "idempotency_conflict", "idempotency key already used with a different request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if err != nil {
		slog.Warn("idempotency check failed", "err", err)
	}
	if found {
		api.Success(w, json.RawMessage(stored), middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.FinalizePeriod(r.Context(), user.TenantID, periodID); err != nil {
		if h.Audit != nil {
			if auditErr := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payroll.finalize.failed", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"error": err.Error()}); auditErr != nil {
				slog.Warn("audit payroll.finalize.failed failed", "err", auditErr)
			}
		}
		if errors.Is(err, payroll.ErrPeriodNotFound) {
			api.Fail(w, http.StatusNotFound, "not_found", "payroll period not found", middleware.GetRequestID(r.Context()))
			return
		}
		if errors.Is(err, payroll.ErrFinalizeInvalidState) || errors.Is(err, payroll.ErrFinalizeNoResults) {
			api.Fail(w, http.StatusBadRequest, "invalid_state", err.Error(), middleware.GetRequestID(r.Context()))
			return
		}
		api.Fail(w, http.StatusInternalServerError, "payroll_finalize_failed", "failed to finalize payroll", middleware.GetRequestID(r.Context()))
		return
	}

	payslips, err := h.Service.ListPayslipIDs(r.Context(), user.TenantID, periodID)
	if err == nil {
		for _, payslip := range payslips {
			fileURL, err := h.Service.GeneratePayslipPDF(r.Context(), user.TenantID, periodID, payslip.EmployeeID, payslip.ID)
			if err != nil {
				slog.Warn("payslip pdf generation failed", "err", err)
			} else if fileURL != "" {
				if err := h.Service.UpdatePayslipFileURL(r.Context(), payslip.ID, fileURL); err != nil {
					slog.Warn("payslip file url update failed", "err", err)
				}
			}
			employeeUserID, err := h.Service.EmployeeUserID(r.Context(), user.TenantID, payslip.EmployeeID)
			if err != nil {
				slog.Warn("payslip employee user lookup failed", "err", err)
			}
			if employeeUserID != "" && h.Notify != nil {
				if err := h.Notify.Create(r.Context(), user.TenantID, employeeUserID, notifications.TypePayslipPublished, "Payslip published", "A new payslip is available for download."); err != nil {
					slog.Warn("payslip notification failed", "err", err)
				}
			}
		}
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payroll.finalize", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		slog.Warn("audit payroll.finalize failed", "err", err)
	}

	response := map[string]string{"status": payroll.PeriodStatusFinalized}
	payload, err := json.Marshal(response)
	if err != nil {
		slog.Warn("finalize response marshal failed", "err", err)
	}
	if err := h.Idem.Save(r.Context(), user.TenantID, user.UserID, "payroll.finalize", idempotencyKey, requestHash, payload); err != nil {
		if errors.Is(err, middleware.ErrIdempotencyConflict) {
			api.Fail(w, http.StatusConflict, "idempotency_conflict", "idempotency key already used with a different request payload", middleware.GetRequestID(r.Context()))
			return
		}
		slog.Warn("idempotency save failed", "err", err)
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
		if id, err := h.Service.FindEmployeeIDByUserID(r.Context(), user.TenantID, user.UserID); err != nil {
			slog.Warn("payslip list employee lookup failed", "err", err)
		} else {
			employeeID = id
		}
	}
	if employeeID == "" {
		api.Success(w, []payroll.Payslip{}, middleware.GetRequestID(r.Context()))
		return
	}

	page := shared.ParsePagination(r, 100, 500)
	total, err := h.Service.CountPayslips(r.Context(), user.TenantID, employeeID)
	if err != nil {
		slog.Warn("payslip count failed", "err", err)
	}
	slips, err := h.Service.ListPayslips(r.Context(), user.TenantID, employeeID, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payslip_list_failed", "failed to list payslips", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
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
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "unable to read CSV payload"},
		})
		return
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	requestHash := middleware.RequestHash(body)
	if idempotencyKey != "" {
		stored, found, err := h.Idem.Check(r.Context(), user.TenantID, user.UserID, "payroll.inputs.import", idempotencyKey, requestHash)
		if errors.Is(err, middleware.ErrIdempotencyConflict) {
			api.Fail(w, http.StatusConflict, "idempotency_conflict", "idempotency key already used with a different request payload", middleware.GetRequestID(r.Context()))
			return
		}
		if err != nil {
			slog.Warn("idempotency check failed", "err", err)
		}
		if found {
			api.Success(w, json.RawMessage(stored), middleware.GetRequestID(r.Context()))
			return
		}
	}

	reader := csv.NewReader(bytes.NewReader(body))
	headers, err := reader.Read()
	if err != nil {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "CSV header row is required"},
		})
		return
	}

	index := map[string]int{}
	for i, header := range headers {
		index[strings.ToLower(strings.TrimSpace(header))] = i
	}

	get := func(row []string, key string) string {
		if idx, ok := index[key]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	validator := shared.NewValidator()
	insertRows := make([]payroll.Input, 0, 32)
	rowNumber := 1
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			validator.Add("rows", "row "+strconv.Itoa(rowNumber+1)+" is malformed")
			break
		}
		rowNumber++

		employeeID := strings.TrimSpace(get(row, "employee_id"))
		if employeeID == "" {
			if email := get(row, "employee_email"); email != "" {
				if id, err := h.Service.FindEmployeeIDByEmail(r.Context(), user.TenantID, email); err != nil {
					validator.Add("rows["+strconv.Itoa(rowNumber)+"].employeeEmail", "employee not found for email "+email)
				} else {
					employeeID = id
				}
			}
		}
		if employeeID == "" {
			validator.Add("rows["+strconv.Itoa(rowNumber)+"].employeeId", "employee_id or resolvable employee_email is required")
		}

		elementID := strings.TrimSpace(get(row, "element_id"))
		if elementID == "" {
			validator.Add("rows["+strconv.Itoa(rowNumber)+"].elementId", "is required")
		}

		parseFloat := func(field, raw string) float64 {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return 0
			}
			value, parseErr := strconv.ParseFloat(raw, 64)
			if parseErr != nil {
				validator.Add("rows["+strconv.Itoa(rowNumber)+"]."+field, "must be a valid number")
				return 0
			}
			return value
		}

		units := parseFloat("units", get(row, "units"))
		rate := parseFloat("rate", get(row, "rate"))
		amount := parseFloat("amount", get(row, "amount"))
		if units < 0 {
			validator.Add("rows["+strconv.Itoa(rowNumber)+"].units", "must be greater than or equal to 0")
		}
		if rate < 0 {
			validator.Add("rows["+strconv.Itoa(rowNumber)+"].rate", "must be greater than or equal to 0")
		}
		if amount < 0 {
			validator.Add("rows["+strconv.Itoa(rowNumber)+"].amount", "must be greater than or equal to 0")
		}

		source := strings.ToLower(strings.TrimSpace(get(row, "source")))
		if source == "" {
			source = payroll.InputSourceImport
		}
		validator.Enum("rows["+strconv.Itoa(rowNumber)+"].source", source, []string{payroll.InputSourceImport, payroll.InputSourceManual}, "must be one of: import, manual")

		if amount == 0 && units > 0 {
			amount = units * rate
		}
		if amount <= 0 {
			validator.Add("rows["+strconv.Itoa(rowNumber)+"].amount", "must be greater than 0")
		}

		insertRows = append(insertRows, payroll.Input{
			EmployeeID: employeeID,
			ElementID:  elementID,
			Units:      units,
			Rate:       rate,
			Amount:     amount,
			Source:     source,
		})
	}
	if validator.Reject(w, middleware.GetRequestID(r.Context())) {
		return
	}

	inserted := 0
	for _, input := range insertRows {
		if err := h.Service.CreateInput(r.Context(), user.TenantID, periodID, input); err != nil {
			api.Fail(w, http.StatusInternalServerError, "payroll_input_failed", "failed to import one or more input rows", middleware.GetRequestID(r.Context()))
			return
		}
		inserted++
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payroll.inputs.import", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"count": inserted}); err != nil {
		slog.Warn("audit payroll.inputs.import failed", "err", err)
	}
	response := map[string]any{"imported": inserted}
	if idempotencyKey != "" {
		encoded, err := json.Marshal(response)
		if err != nil {
			slog.Warn("idempotency response marshal failed", "err", err)
		} else if err := h.Idem.Save(r.Context(), user.TenantID, user.UserID, "payroll.inputs.import", idempotencyKey, requestHash, encoded); err != nil {
			if errors.Is(err, middleware.ErrIdempotencyConflict) {
				api.Fail(w, http.StatusConflict, "idempotency_conflict", "idempotency key already used with a different request payload", middleware.GetRequestID(r.Context()))
				return
			}
			slog.Warn("idempotency save failed", "err", err)
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
	employeeFilter := ""
	if user.RoleName == auth.RoleEmployee {
		if employeeID, err := h.Service.FindEmployeeIDByUserID(r.Context(), user.TenantID, user.UserID); err != nil {
			slog.Warn("adjustments list employee lookup failed", "err", err)
		} else {
			employeeFilter = employeeID
		}
	}

	page := shared.ParsePagination(r, 100, 500)
	adjustments, total, err := h.Service.ListAdjustments(r.Context(), user.TenantID, periodID, employeeFilter, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "adjustments_list_failed", "failed to list adjustments", middleware.GetRequestID(r.Context()))
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
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
		EmployeeID    string  `json:"employeeId"`
		Description   string  `json:"description"`
		Amount        float64 `json:"amount"`
		EffectiveDate string  `json:"effectiveDate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "must be valid JSON"},
		})
		return
	}

	payload.EmployeeID = strings.TrimSpace(payload.EmployeeID)
	payload.Description = strings.TrimSpace(payload.Description)
	validator := shared.NewValidator()
	validator.Required("employeeId", payload.EmployeeID, "is required")
	validator.Required("description", payload.Description, "is required")
	if payload.Amount == 0 {
		validator.Add("amount", "must be non-zero")
	}

	var effectiveDate any
	if payload.EffectiveDate != "" {
		parsed, ok := validator.Date("effectiveDate", payload.EffectiveDate)
		if ok {
			effectiveDate = parsed
		}
	}
	if validator.Reject(w, middleware.GetRequestID(r.Context())) {
		return
	}

	id, err := h.Service.CreateAdjustment(r.Context(), user.TenantID, periodID, payload.EmployeeID, payload.Description, payload.Amount, effectiveDate)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "adjustment_create_failed", "failed to create adjustment", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payroll.adjustment.create", "payroll_adjustment", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit payroll.adjustment.create failed", "err", err)
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
	summary, err := h.Service.PeriodSummary(r.Context(), user.TenantID, periodID)
	if err != nil {
		slog.Warn("period summary totals query failed", "err", err)
	}
	api.Success(w, summary, middleware.GetRequestID(r.Context()))
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
		slog.Warn("reopen payload decode failed", "err", err)
	}
	if strings.TrimSpace(payload.Reason) == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "reopen reason required", middleware.GetRequestID(r.Context()))
		return
	}

	currentStatus, err := h.Service.PeriodStatus(r.Context(), user.TenantID, periodID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "payroll period not found", middleware.GetRequestID(r.Context()))
		return
	}
	if currentStatus != payroll.PeriodStatusFinalized {
		api.Fail(w, http.StatusBadRequest, "invalid_state", "only finalized periods can be reopened", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.UpdatePeriodStatus(r.Context(), user.TenantID, periodID, payroll.PeriodStatusDraft); err != nil {
		api.Fail(w, http.StatusInternalServerError, "payroll_reopen_failed", "failed to reopen payroll", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Service.DeleteResultsForPeriod(r.Context(), user.TenantID, periodID); err != nil {
		slog.Warn("payroll results delete failed", "err", err)
	}
	if err := h.Service.DeletePayslipsForPeriod(r.Context(), user.TenantID, periodID); err != nil {
		slog.Warn("payslips delete failed", "err", err)
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payroll.reopen", "payroll_period", periodID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit payroll.reopen failed", "err", err)
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
	rows, err := h.Service.RegisterRows(r.Context(), user.TenantID, periodID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "export_failed", "failed to export register", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=payroll-register.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"employee_id", "first_name", "last_name", "gross", "deductions", "net", "currency"}); err != nil {
		slog.Warn("export register header write failed", "err", err)
	}
	for _, row := range rows {
		if err := writer.Write([]string{row.EmployeeID, row.FirstName, row.LastName, fmt.Sprintf("%.2f", row.Gross), fmt.Sprintf("%.2f", row.Deductions), fmt.Sprintf("%.2f", row.Net), row.Currency}); err != nil {
			slog.Warn("export register row write failed", "err", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		slog.Warn("export register flush failed", "err", err)
	}
}

func (h *Handler) handleExportJournal(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	periodID := chi.URLParam(r, "periodID")
	gross, deductions, net, err := h.Service.PeriodTotals(r.Context(), user.TenantID, periodID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "export_failed", "failed to export journal", middleware.GetRequestID(r.Context()))
		return
	}

	expenseAccount := "Payroll Expense"
	deductionAccount := "Payroll Deductions"
	cashAccount := "Payroll Cash"
	headers := []string{"account", "debit", "credit"}
	if templateID := r.URL.Query().Get("templateId"); templateID != "" {
		if cfg, err := h.Service.JournalTemplateConfig(r.Context(), user.TenantID, templateID); err == nil {
			if val, ok := cfg["expenseAccount"].(string); ok && val != "" {
				expenseAccount = val
			}
			if val, ok := cfg["deductionAccount"].(string); ok && val != "" {
				deductionAccount = val
			}
			if val, ok := cfg["cashAccount"].(string); ok && val != "" {
				cashAccount = val
			}
			if rawHeaders, ok := cfg["headers"].([]any); ok && len(rawHeaders) > 0 {
				headers = []string{}
				for _, header := range rawHeaders {
					if hs, ok := header.(string); ok {
						headers = append(headers, hs)
					}
				}
				if len(headers) == 0 {
					headers = []string{"account", "debit", "credit"}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=payroll-journal.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write(headers); err != nil {
		slog.Warn("export journal header write failed", "err", err)
	}
	if err := writer.Write([]string{expenseAccount, fmt.Sprintf("%.2f", gross), ""}); err != nil {
		slog.Warn("export journal expense row write failed", "err", err)
	}
	if err := writer.Write([]string{deductionAccount, "", fmt.Sprintf("%.2f", deductions)}); err != nil {
		slog.Warn("export journal deductions row write failed", "err", err)
	}
	if err := writer.Write([]string{cashAccount, "", fmt.Sprintf("%.2f", net)}); err != nil {
		slog.Warn("export journal cash row write failed", "err", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		slog.Warn("export journal flush failed", "err", err)
	}
}

func (h *Handler) handleDownloadPayslip(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	payslipID := chi.URLParam(r, "payslipID")
	employeeID, fileURL, err := h.Service.PayslipInfo(r.Context(), user.TenantID, payslipID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "payslip not found", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName != auth.RoleHR {
		selfEmployeeID, err := h.Service.FindEmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("payslip download self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || employeeID != selfEmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if fileURL == "" {
		periodID, err := h.Service.PayslipPeriodID(r.Context(), user.TenantID, payslipID)
		if err != nil {
			slog.Warn("payslip period lookup failed", "err", err)
		}
		fileURL, err = h.Service.GeneratePayslipPDF(r.Context(), user.TenantID, periodID, employeeID, payslipID)
		if err != nil {
			slog.Warn("payslip pdf generation failed", "err", err)
		}
		if fileURL != "" {
			if err := h.Service.UpdatePayslipFileURL(r.Context(), payslipID, fileURL); err != nil {
				slog.Warn("payslip file url update failed", "err", err)
			}
		}
	}

	if fileURL == "" {
		api.Fail(w, http.StatusInternalServerError, "payslip_missing", "payslip not available", middleware.GetRequestID(r.Context()))
		return
	}

	if h.Crypto != nil && h.Crypto.Configured() && strings.HasSuffix(fileURL, ".enc") {
		contents, err := os.ReadFile(fileURL)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "payslip_missing", "payslip not available", middleware.GetRequestID(r.Context()))
			return
		}
		plain, err := h.Crypto.Decrypt(contents)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "payslip_missing", "payslip not available", middleware.GetRequestID(r.Context()))
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=payslip.pdf")
		if _, err := w.Write(plain); err != nil {
			slog.Warn("payslip download write failed", "err", err)
		}
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
	employeeID, periodID, err := h.Service.PayslipEmployeePeriod(r.Context(), user.TenantID, payslipID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "payslip not found", middleware.GetRequestID(r.Context()))
		return
	}

	fileURL, err := h.Service.GeneratePayslipPDF(r.Context(), user.TenantID, periodID, employeeID, payslipID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "payslip_generate_failed", "failed to regenerate payslip", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Service.UpdatePayslipFileURL(r.Context(), payslipID, fileURL); err != nil {
		slog.Warn("payslip regenerate update failed", "err", err)
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "payslip.regenerate", "payslip", payslipID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, nil); err != nil {
		slog.Warn("audit payslip.regenerate failed", "err", err)
	}
	api.Success(w, map[string]string{"status": "regenerated"}, middleware.GetRequestID(r.Context()))
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
