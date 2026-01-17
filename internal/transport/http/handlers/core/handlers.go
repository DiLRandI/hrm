package corehandler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

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
	Store *core.Store
}

func NewHandler(store *core.Store) *Handler {
	return &Handler{Store: store}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/me", h.handleMe)
	r.Get("/org/chart", h.handleOrgChart)
	r.Route("/employees", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermEmployeesRead, h.Store)).Get("/", h.handleListEmployees)
		r.With(middleware.RequirePermission(auth.PermEmployeesWrite, h.Store)).Post("/", h.handleCreateEmployee)
		r.Route("/{employeeID}", func(r chi.Router) {
			r.With(middleware.RequirePermission(auth.PermEmployeesRead, h.Store)).Get("/", h.handleGetEmployee)
			r.Put("/", h.handleUpdateEmployee)
			r.With(middleware.RequirePermission(auth.PermEmployeesRead, h.Store)).Get("/manager-history", h.handleManagerHistory)
		})
	})
	r.Route("/departments", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermOrgRead, h.Store)).Get("/", h.handleListDepartments)
		r.With(middleware.RequirePermission(auth.PermOrgWrite, h.Store)).Post("/", h.handleCreateDepartment)
		r.With(middleware.RequirePermission(auth.PermOrgWrite, h.Store)).Put("/{departmentID}", h.handleUpdateDepartment)
		r.With(middleware.RequirePermission(auth.PermOrgWrite, h.Store)).Delete("/{departmentID}", h.handleDeleteDepartment)
	})
	r.With(middleware.RequirePermission(auth.PermOrgRead, h.Store)).Get("/permissions", h.handleListPermissions)
	r.With(middleware.RequirePermission(auth.PermOrgRead, h.Store)).Get("/roles", h.handleListRoles)
	r.With(middleware.RequirePermission(auth.PermOrgWrite, h.Store)).Put("/roles/{roleID}", h.handleUpdateRolePermissions)
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	exists, err := h.Store.UserExists(r.Context(), user.TenantID, user.UserID)
	if err != nil || !exists {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	emp, err := h.Store.GetEmployeeByUserID(r.Context(), user.TenantID, user.UserID)
	if err == nil {
		isSelf := true
		core.FilterEmployeeFields(emp, user, isSelf, false)
	}

	var mfaEnabled bool
	if err := h.Store.DB.QueryRow(r.Context(), "SELECT mfa_enabled FROM users WHERE id = $1", user.UserID).Scan(&mfaEnabled); err != nil {
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

	employees, err := h.Store.ListEmployees(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "employee_list_failed", "failed to list employees", middleware.GetRequestID(r.Context()))
		return
	}

	var managerEmployeeID string
	if user.RoleName == auth.RoleManager {
		managerEmp, err := h.Store.GetEmployeeByUserID(r.Context(), user.TenantID, user.UserID)
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
	emp, err := h.Store.GetEmployee(r.Context(), user.TenantID, employeeID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "employee not found", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := h.Store.DB.Exec(r.Context(), `
    INSERT INTO access_logs (tenant_id, actor_user_id, employee_id, fields, request_id)
    VALUES ($1,$2,$3,$4,$5)
  `, user.TenantID, user.UserID, employeeID, []string{"employee_profile"}, middleware.GetRequestID(r.Context())); err != nil {
		slog.Warn("access log insert failed", "err", err)
	}

	managerEmp, err := h.Store.GetEmployeeByUserID(r.Context(), user.TenantID, user.UserID)
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
	if payload.Status == "" {
		payload.Status = core.EmployeeStatusActive
	}

	id, err := h.Store.CreateEmployee(r.Context(), user.TenantID, payload)
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
		if _, err := h.Store.DB.Exec(r.Context(), `
    INSERT INTO manager_relations (employee_id, manager_id, start_date)
    VALUES ($1, $2, CURRENT_DATE)
  `, id, payload.ManagerID); err != nil {
			slog.Warn("manager relation insert failed", "err", err)
		}
	}

	if err := audit.New(h.Store.DB).Record(r.Context(), user.TenantID, user.UserID, "core.employee.create", "employee", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit core.employee.create failed", "err", err)
	}

	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleUpdateEmployee(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID := chi.URLParam(r, "employeeID")
	existing, err := h.Store.GetEmployee(r.Context(), user.TenantID, employeeID)
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

	if err := h.Store.UpdateEmployee(r.Context(), user.TenantID, employeeID, payload); err != nil {
		api.Fail(w, http.StatusInternalServerError, "employee_update_failed", "failed to update employee", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName == auth.RoleHR && previousManagerID != payload.ManagerID {
		if _, err := h.Store.DB.Exec(r.Context(), `
      UPDATE manager_relations
      SET end_date = CURRENT_DATE
      WHERE employee_id = $1 AND end_date IS NULL
    `, employeeID); err != nil {
			slog.Warn("manager relation close failed", "err", err)
		}
		if payload.ManagerID != "" {
			if _, err := h.Store.DB.Exec(r.Context(), `
      INSERT INTO manager_relations (employee_id, manager_id, start_date)
      VALUES ($1, $2, CURRENT_DATE)
    `, employeeID, payload.ManagerID); err != nil {
				slog.Warn("manager relation insert failed", "err", err)
			}
		}

		if err := audit.New(h.Store.DB).Record(r.Context(), user.TenantID, user.UserID, "core.employee.manager_change", "employee", employeeID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), map[string]any{"managerId": previousManagerID}, map[string]any{"managerId": payload.ManagerID}); err != nil {
			slog.Warn("audit core.employee.manager_change failed", "err", err)
		}
	}

	if err := audit.New(h.Store.DB).Record(r.Context(), user.TenantID, user.UserID, "core.employee.update", "employee", employeeID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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

	var total int
	if err := h.Store.DB.QueryRow(r.Context(), "SELECT COUNT(1) FROM departments WHERE tenant_id = $1", user.TenantID).Scan(&total); err != nil {
		slog.Warn("department count failed", "err", err)
	}

	rows, err := h.Store.DB.Query(r.Context(), `
    SELECT id, name, parent_id, manager_id, created_at
    FROM departments
    WHERE tenant_id = $1
    ORDER BY name
    LIMIT $2 OFFSET $3
  `, user.TenantID, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "department_list_failed", "failed to list departments", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var departments []core.Department
	for rows.Next() {
		var dep core.Department
		if err := rows.Scan(&dep.ID, &dep.Name, &dep.ParentID, &dep.ManagerID, &dep.CreatedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "department_list_failed", "failed to list departments", middleware.GetRequestID(r.Context()))
			return
		}
		departments = append(departments, dep)
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
	err := h.Store.DB.QueryRow(r.Context(), `
    INSERT INTO departments (tenant_id, name, parent_id, manager_id)
    VALUES ($1, $2, $3, $4)
    RETURNING id
  `, user.TenantID, payload.Name, nullIfEmpty(payload.ParentID), nullIfEmpty(payload.ManagerID)).Scan(&id)
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

	cmd, err := h.Store.DB.Exec(r.Context(), `
    UPDATE departments
    SET name = $1, parent_id = $2, manager_id = $3
    WHERE tenant_id = $4 AND id = $5
  `, payload.Name, nullIfEmpty(payload.ParentID), nullIfEmpty(payload.ManagerID), user.TenantID, departmentID)
	if err != nil || cmd.RowsAffected() == 0 {
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
	var dependent int
	if err := h.Store.DB.QueryRow(r.Context(), `
    SELECT COUNT(1) FROM employees WHERE tenant_id = $1 AND department_id = $2
  `, user.TenantID, departmentID).Scan(&dependent); err == nil && dependent > 0 {
		api.Fail(w, http.StatusBadRequest, "department_in_use", "department has assigned employees", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := h.Store.DB.Exec(r.Context(), `
    DELETE FROM departments WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, departmentID); err != nil {
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

	query := `
    SELECT id, first_name, last_name, COALESCE(manager_id::text,''), COALESCE(department_id::text,'')
    FROM employees
    WHERE tenant_id = $1`
	args := []any{user.TenantID}
	if user.RoleName == auth.RoleEmployee || user.RoleName == auth.RoleManager {
		var employeeID string
		if err := h.Store.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err == nil && employeeID != "" {
			query += " AND (id = $2 OR manager_id = $2)"
			args = append(args, employeeID)
		}
	}
	query += " ORDER BY last_name, first_name"

	rows, err := h.Store.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "org_chart_failed", "failed to load org chart", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var nodes []map[string]any
	for rows.Next() {
		var id, first, last, managerID, departmentID string
		if err := rows.Scan(&id, &first, &last, &managerID, &departmentID); err != nil {
			api.Fail(w, http.StatusInternalServerError, "org_chart_failed", "failed to load org chart", middleware.GetRequestID(r.Context()))
			return
		}
		nodes = append(nodes, map[string]any{
			"id":           id,
			"name":         first + " " + last,
			"managerId":    managerID,
			"departmentId": departmentID,
		})
	}
	api.Success(w, nodes, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleManagerHistory(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUser(r.Context()); !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID := chi.URLParam(r, "employeeID")
	rows, err := h.Store.DB.Query(r.Context(), `
    SELECT mr.manager_id, COALESCE(m.first_name,''), COALESCE(m.last_name,''), mr.start_date, mr.end_date
    FROM manager_relations mr
    LEFT JOIN employees m ON mr.manager_id = m.id
    WHERE mr.employee_id = $1
    ORDER BY mr.start_date DESC
  `, employeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "manager_history_failed", "failed to load manager history", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()
	var history []map[string]any
	for rows.Next() {
		var managerID, first, last string
		var startDate, endDate any
		if err := rows.Scan(&managerID, &first, &last, &startDate, &endDate); err != nil {
			api.Fail(w, http.StatusInternalServerError, "manager_history_failed", "failed to load manager history", middleware.GetRequestID(r.Context()))
			return
		}
		history = append(history, map[string]any{
			"managerId": managerID,
			"name":      first + " " + last,
			"startDate": startDate,
			"endDate":   endDate,
		})
	}
	api.Success(w, history, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Store.DB.Query(r.Context(), `
    SELECT key, COALESCE(description,'')
    FROM permissions
    ORDER BY key
  `)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "permission_list_failed", "failed to list permissions", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var out []map[string]string
	for rows.Next() {
		var key, desc string
		if err := rows.Scan(&key, &desc); err != nil {
			api.Fail(w, http.StatusInternalServerError, "permission_list_failed", "failed to list permissions", middleware.GetRequestID(r.Context()))
			return
		}
		out = append(out, map[string]string{"key": key, "description": desc})
	}
	api.Success(w, out, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListRoles(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	rows, err := h.Store.DB.Query(r.Context(), `
    SELECT r.id,
           r.name,
           COALESCE(array_agg(p.key) FILTER (WHERE p.key IS NOT NULL), '{}') AS permissions
    FROM roles r
    LEFT JOIN role_permissions rp ON rp.role_id = r.id
    LEFT JOIN permissions p ON rp.permission_id = p.id
    WHERE r.tenant_id = $1
    GROUP BY r.id
    ORDER BY r.name
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "role_list_failed", "failed to list roles", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var roles []map[string]any
	for rows.Next() {
		var id, name string
		var perms []string
		if err := rows.Scan(&id, &name, &perms); err != nil {
			api.Fail(w, http.StatusInternalServerError, "role_list_failed", "failed to list roles", middleware.GetRequestID(r.Context()))
			return
		}
		roles = append(roles, map[string]any{"id": id, "name": name, "permissions": perms})
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
	var roleTenant string
	if err := h.Store.DB.QueryRow(r.Context(), "SELECT tenant_id::text FROM roles WHERE id = $1", roleID).Scan(&roleTenant); err != nil || roleTenant != user.TenantID {
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

	tx, err := h.Store.DB.Begin(r.Context())
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "role_update_failed", "failed to update role", middleware.GetRequestID(r.Context()))
		return
	}
	defer tx.Rollback(r.Context())

	if _, err := tx.Exec(r.Context(), "DELETE FROM role_permissions WHERE role_id = $1", roleID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "role_update_failed", "failed to update role", middleware.GetRequestID(r.Context()))
		return
	}

	if len(payload.Permissions) > 0 {
		rows, err := tx.Query(r.Context(), `
      SELECT id, key FROM permissions WHERE key = ANY($1)
    `, payload.Permissions)
		if err != nil {
			api.Fail(w, http.StatusInternalServerError, "role_update_failed", "failed to update role", middleware.GetRequestID(r.Context()))
			return
		}
		var permissionIDs []string
		for rows.Next() {
			var id, key string
			if err := rows.Scan(&id, &key); err != nil {
				rows.Close()
				api.Fail(w, http.StatusInternalServerError, "role_update_failed", "failed to update role", middleware.GetRequestID(r.Context()))
				return
			}
			permissionIDs = append(permissionIDs, id)
		}
		rows.Close()
		for _, permID := range permissionIDs {
			if _, err := tx.Exec(r.Context(), "INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2)", roleID, permID); err != nil {
				api.Fail(w, http.StatusInternalServerError, "role_update_failed", "failed to update role", middleware.GetRequestID(r.Context()))
				return
			}
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
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
