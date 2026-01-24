package corehandler

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	Service *core.Service
	Audit   *audit.Service
}

func NewHandler(service *core.Service, auditSvc *audit.Service) *Handler {
	return &Handler{Service: service, Audit: auditSvc}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/me", h.handleMe)
	r.Get("/org/chart", h.handleOrgChart)
	r.Route("/employees", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermEmployeesRead, h.Service)).Get("/", h.handleListEmployees)
		r.With(middleware.RequirePermission(auth.PermEmployeesWrite, h.Service)).Post("/", h.handleCreateEmployee)
		r.Route("/{employeeID}", func(r chi.Router) {
			r.With(middleware.RequirePermission(auth.PermEmployeesRead, h.Service)).Get("/", h.handleGetEmployee)
			r.Put("/", h.handleUpdateEmployee)
			r.With(middleware.RequirePermission(auth.PermEmployeesRead, h.Service)).Get("/manager-history", h.handleManagerHistory)
		})
	})
	r.Route("/departments", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermOrgRead, h.Service)).Get("/", h.handleListDepartments)
		r.With(middleware.RequirePermission(auth.PermOrgWrite, h.Service)).Post("/", h.handleCreateDepartment)
		r.With(middleware.RequirePermission(auth.PermOrgWrite, h.Service)).Put("/{departmentID}", h.handleUpdateDepartment)
		r.With(middleware.RequirePermission(auth.PermOrgWrite, h.Service)).Delete("/{departmentID}", h.handleDeleteDepartment)
	})
	r.With(middleware.RequirePermission(auth.PermOrgRead, h.Service)).Get("/permissions", h.handleListPermissions)
	r.With(middleware.RequirePermission(auth.PermOrgRead, h.Service)).Get("/roles", h.handleListRoles)
	r.With(middleware.RequirePermission(auth.PermOrgWrite, h.Service)).Put("/roles/{roleID}", h.handleUpdateRolePermissions)
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	exists, err := h.Service.UserExists(r.Context(), user.TenantID, user.UserID)
	if err != nil || !exists {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	emp, err := h.Service.GetEmployeeByUserID(r.Context(), user.TenantID, user.UserID)
	if err == nil {
		isSelf := true
		core.FilterEmployeeFields(emp, user, isSelf, false)
	}

	var mfaEnabled bool
	if mfaEnabled, err = h.Service.MFAEnabled(r.Context(), user.UserID); err != nil {
		slog.Warn("mfa status lookup failed", "err", err)
	}

	api.Success(w, map[string]any{
		"user": map[string]any{
			"id":         user.UserID,
			"tenantId":   user.TenantID,
			"roleId":     user.RoleID,
			"role":       user.RoleName,
			"mfaEnabled": mfaEnabled,
		},
		"employee": emp,
	}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListEmployees(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	employees, err := h.Service.ListEmployees(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "employee_list_failed", "failed to list employees", middleware.GetRequestID(r.Context()))
		return
	}

	var managerEmployeeID string
	if user.RoleName == auth.RoleManager {
		managerEmp, err := h.Service.GetEmployeeByUserID(r.Context(), user.TenantID, user.UserID)
		if err == nil {
			managerEmployeeID = managerEmp.ID
		}
	}

	filtered := make([]core.Employee, 0, len(employees))
	for _, emp := range employees {
		if user.RoleName == auth.RoleManager && managerEmployeeID != "" && emp.ManagerID != managerEmployeeID && emp.UserID != user.UserID {
			continue
		}
		if user.RoleName == auth.RoleEmployee && emp.UserID != user.UserID {
			continue
		}

		isSelf := emp.UserID == user.UserID
		isManager := emp.ManagerID == managerEmployeeID
		core.FilterEmployeeFields(&emp, user, isSelf, isManager)
		filtered = append(filtered, emp)
	}

	page := shared.ParsePagination(r, 100, 500)
	total := len(filtered)
	start := min(page.Offset, total)
	end := min(start+page.Limit, total)
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	api.Success(w, filtered[start:end], middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleGetEmployee(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID := chi.URLParam(r, "employeeID")
	emp, err := h.Service.GetEmployee(r.Context(), user.TenantID, employeeID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "employee not found", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.InsertAccessLog(r.Context(), user.TenantID, user.UserID, employeeID, middleware.GetRequestID(r.Context()), []string{"employee_profile"}); err != nil {
		slog.Warn("access log insert failed", "err", err)
	}

	managerEmp, err := h.Service.GetEmployeeByUserID(r.Context(), user.TenantID, user.UserID)
	if err != nil {
		slog.Warn("manager lookup failed", "err", err)
	}
	isSelf := emp.UserID == user.UserID
	isManager := managerEmp != nil && emp.ManagerID == managerEmp.ID
	if user.RoleName == auth.RoleEmployee && !isSelf {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName == auth.RoleManager && !isSelf && !isManager {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
		return
	}

	core.FilterEmployeeFields(emp, user, isSelf, isManager)
	api.Success(w, emp, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateEmployee(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload core.Employee
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if strings.TrimSpace(payload.Email) == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee email required", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.Status == "" {
		payload.Status = core.EmployeeStatusActive
	}

	tempPassword, err := generateTempPassword()
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "password_generate_failed", "failed to generate temporary password", middleware.GetRequestID(r.Context()))
		return
	}

	id, _, err := h.Service.CreateEmployeeWithUser(r.Context(), user.TenantID, payload, tempPassword)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			api.Fail(w, http.StatusConflict, "employee_exists", "employee email already exists", middleware.GetRequestID(r.Context()))
			return
		}
		api.Fail(w, http.StatusInternalServerError, "employee_create_failed", "failed to create employee", middleware.GetRequestID(r.Context()))
		return
	}

	if payload.ManagerID != "" {
		if err := h.Service.CreateManagerRelation(r.Context(), id, payload.ManagerID); err != nil {
			slog.Warn("manager relation insert failed", "err", err)
		}
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "core.employee.create", "employee", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit core.employee.create failed", "err", err)
	}

	api.Created(w, map[string]string{"id": id, "tempPassword": tempPassword}, middleware.GetRequestID(r.Context()))
}

const tempPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
const tempPasswordLength = 12

func generateTempPassword() (string, error) {
	out := make([]byte, tempPasswordLength)
	max := big.NewInt(int64(len(tempPasswordAlphabet)))
	for i := 0; i < tempPasswordLength; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = tempPasswordAlphabet[n.Int64()]
	}
	return string(out), nil
}

func (h *Handler) handleUpdateEmployee(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID := chi.URLParam(r, "employeeID")
	existing, err := h.Service.GetEmployee(r.Context(), user.TenantID, employeeID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "employee not found", middleware.GetRequestID(r.Context()))
		return
	}
	previousManagerID := existing.ManagerID

	var payload core.Employee
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName != auth.RoleHR {
		if existing.UserID != user.UserID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
		payload.EmployeeNumber = existing.EmployeeNumber
		payload.FirstName = existing.FirstName
		payload.LastName = existing.LastName
		payload.Email = existing.Email
		payload.NationalID = existing.NationalID
		payload.BankAccount = existing.BankAccount
		payload.Salary = existing.Salary
		payload.Currency = existing.Currency
		payload.EmploymentType = existing.EmploymentType
		payload.DepartmentID = existing.DepartmentID
		payload.ManagerID = existing.ManagerID
		payload.PayGroupID = existing.PayGroupID
		payload.StartDate = existing.StartDate
		payload.EndDate = existing.EndDate
		payload.Status = existing.Status
	}

	if err := h.Service.UpdateEmployee(r.Context(), user.TenantID, employeeID, payload); err != nil {
		api.Fail(w, http.StatusInternalServerError, "employee_update_failed", "failed to update employee", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName == auth.RoleHR && previousManagerID != payload.ManagerID {
		if err := h.Service.CloseManagerRelations(r.Context(), employeeID); err != nil {
			slog.Warn("manager relation close failed", "err", err)
		}
		if payload.ManagerID != "" {
			if err := h.Service.CreateManagerRelation(r.Context(), employeeID, payload.ManagerID); err != nil {
				slog.Warn("manager relation insert failed", "err", err)
			}
		}

		if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "core.employee.manager_change", "employee", employeeID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), map[string]any{"managerId": previousManagerID}, map[string]any{"managerId": payload.ManagerID}); err != nil {
			slog.Warn("audit core.employee.manager_change failed", "err", err)
		}
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "core.employee.update", "employee", employeeID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit core.employee.update failed", "err", err)
	}

	api.Success(w, map[string]string{"id": employeeID}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListDepartments(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	page := shared.ParsePagination(r, 100, 500)

	var err error
	var total int
	if total, err = h.Service.DepartmentCount(r.Context(), user.TenantID); err != nil {
		slog.Warn("department count failed", "err", err)
	}

	departments, err := h.Service.ListDepartments(r.Context(), user.TenantID, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "department_list_failed", "failed to list departments", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	api.Success(w, departments, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateDepartment(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload core.Department
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	id, err := h.Service.CreateDepartment(r.Context(), user.TenantID, payload)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "department_create_failed", "failed to create department", middleware.GetRequestID(r.Context()))
		return
	}

	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleUpdateDepartment(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	departmentID := chi.URLParam(r, "departmentID")
	var payload core.Department
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.Name == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "name is required", middleware.GetRequestID(r.Context()))
		return
	}

	updated, err := h.Service.UpdateDepartment(r.Context(), user.TenantID, departmentID, payload)
	if err != nil || !updated {
		api.Fail(w, http.StatusInternalServerError, "department_update_failed", "failed to update department", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, map[string]string{"id": departmentID}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleDeleteDepartment(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	departmentID := chi.URLParam(r, "departmentID")
	dependent, err := h.Service.DepartmentHasEmployees(r.Context(), user.TenantID, departmentID)
	if err == nil && dependent {
		api.Fail(w, http.StatusBadRequest, "department_in_use", "department has assigned employees", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.DeleteDepartment(r.Context(), user.TenantID, departmentID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "department_delete_failed", "failed to delete department", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, map[string]string{"status": "deleted"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleOrgChart(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID := ""
	if user.RoleName == auth.RoleEmployee || user.RoleName == auth.RoleManager {
		if id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID); err == nil {
			employeeID = id
		}
	}
	nodes, err := h.Service.OrgChartNodes(r.Context(), user.TenantID, employeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "org_chart_failed", "failed to load org chart", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, nodes, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleManagerHistory(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUser(r.Context()); !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID := chi.URLParam(r, "employeeID")
	history, err := h.Service.ManagerHistory(r.Context(), employeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "manager_history_failed", "failed to load manager history", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, history, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	out, err := h.Service.ListPermissions(r.Context())
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "permission_list_failed", "failed to list permissions", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, out, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListRoles(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	roles, err := h.Service.ListRoles(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "role_list_failed", "failed to list roles", middleware.GetRequestID(r.Context()))
		return
	}
	api.Success(w, roles, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleUpdateRolePermissions(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR && user.RoleName != auth.RoleSystemAdmin {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr or system admin required", middleware.GetRequestID(r.Context()))
		return
	}

	roleID := chi.URLParam(r, "roleID")
	roleTenant, err := h.Service.RoleTenant(r.Context(), roleID)
	if err != nil || roleTenant != user.TenantID {
		api.Fail(w, http.StatusForbidden, "forbidden", "role not found", middleware.GetRequestID(r.Context()))
		return
	}
	var payload struct {
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.UpdateRolePermissions(r.Context(), roleID, payload.Permissions); err != nil {
		api.Fail(w, http.StatusInternalServerError, "role_update_failed", "failed to update role", middleware.GetRequestID(r.Context()))
		return
	}

	api.Success(w, map[string]string{"status": "updated"}, middleware.GetRequestID(r.Context()))
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
