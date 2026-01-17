package corehandler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

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
	r.Route("/employees", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermEmployeesRead, h.Store)).Get("/", h.handleListEmployees)
		r.With(middleware.RequirePermission(auth.PermEmployeesWrite, h.Store)).Post("/", h.handleCreateEmployee)
		r.Route("/{employeeID}", func(r chi.Router) {
			r.With(middleware.RequirePermission(auth.PermEmployeesRead, h.Store)).Get("/", h.handleGetEmployee)
			r.Put("/", h.handleUpdateEmployee)
		})
	})
	r.Route("/departments", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermOrgRead, h.Store)).Get("/", h.handleListDepartments)
		r.With(middleware.RequirePermission(auth.PermOrgWrite, h.Store)).Post("/", h.handleCreateDepartment)
	})
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

	api.Success(w, map[string]any{
		"user": map[string]string{
			"id":       user.UserID,
			"tenantId": user.TenantID,
			"roleId":   user.RoleID,
			"role":     user.RoleName,
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

	api.Success(w, filtered, middleware.GetRequestID(r.Context()))
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
		log.Printf("access log insert failed: %v", err)
	}

	managerEmp, err := h.Store.GetEmployeeByUserID(r.Context(), user.TenantID, user.UserID)
	if err != nil {
		log.Printf("manager lookup failed: %v", err)
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
			log.Printf("manager relation insert failed: %v", err)
		}
	}

	if err := audit.New(h.Store.DB).Record(r.Context(), user.TenantID, user.UserID, "core.employee.create", "employee", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		log.Printf("audit core.employee.create failed: %v", err)
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
			log.Printf("manager relation close failed: %v", err)
		}
		if payload.ManagerID != "" {
			if _, err := h.Store.DB.Exec(r.Context(), `
      INSERT INTO manager_relations (employee_id, manager_id, start_date)
      VALUES ($1, $2, CURRENT_DATE)
    `, employeeID, payload.ManagerID); err != nil {
				log.Printf("manager relation insert failed: %v", err)
			}
		}

		if err := audit.New(h.Store.DB).Record(r.Context(), user.TenantID, user.UserID, "core.employee.manager_change", "employee", employeeID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), map[string]any{"managerId": previousManagerID}, map[string]any{"managerId": payload.ManagerID}); err != nil {
			log.Printf("audit core.employee.manager_change failed: %v", err)
		}
	}

	if err := audit.New(h.Store.DB).Record(r.Context(), user.TenantID, user.UserID, "core.employee.update", "employee", employeeID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		log.Printf("audit core.employee.update failed: %v", err)
	}

	api.Success(w, map[string]string{"id": employeeID}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListDepartments(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.Store.DB.Query(r.Context(), `
    SELECT id, name, parent_id, manager_id, created_at
    FROM departments
    WHERE tenant_id = $1
    ORDER BY name
  `, user.TenantID)
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

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
