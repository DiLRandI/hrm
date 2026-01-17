package core

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"hrm/internal/api"
	"hrm/internal/middleware"
)

type Handler struct {
	Store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{Store: store}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/me", h.handleMe)
	r.Route("/employees", func(r chi.Router) {
		r.Get("/", h.handleListEmployees)
		r.Post("/", h.handleCreateEmployee)
		r.Route("/{employeeID}", func(r chi.Router) {
			r.Get("/", h.handleGetEmployee)
			r.Put("/", h.handleUpdateEmployee)
		})
	})
	r.Route("/departments", func(r chi.Router) {
		r.Get("/", h.handleListDepartments)
		r.Post("/", h.handleCreateDepartment)
	})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	emp, err := h.Store.GetEmployeeByUserID(r.Context(), user.TenantID, user.UserID)
	if err == nil {
		isSelf := true
		FilterEmployeeFields(emp, user, isSelf, false)
	}

	api.Success(w, map[string]any{
		"user":     user,
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
	if user.RoleName == "Manager" {
		managerEmp, err := h.Store.GetEmployeeByUserID(r.Context(), user.TenantID, user.UserID)
		if err == nil {
			managerEmployeeID = managerEmp.ID
		}
	}

	filtered := make([]Employee, 0, len(employees))
	for _, emp := range employees {
		if user.RoleName == "Manager" && managerEmployeeID != "" && emp.ManagerID != managerEmployeeID && emp.UserID != user.UserID {
			continue
		}
		if user.RoleName == "Employee" && emp.UserID != user.UserID {
			continue
		}

		isSelf := emp.UserID == user.UserID
		isManager := emp.ManagerID == managerEmployeeID
		FilterEmployeeFields(&emp, user, isSelf, isManager)
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

	_, _ = h.Store.DB.Exec(r.Context(), `
    INSERT INTO access_logs (tenant_id, actor_user_id, employee_id, fields, request_id)
    VALUES ($1,$2,$3,$4,$5)
  `, user.TenantID, user.UserID, employeeID, []string{"employee_profile"}, middleware.GetRequestID(r.Context()))

	managerEmp, _ := h.Store.GetEmployeeByUserID(r.Context(), user.TenantID, user.UserID)
	isSelf := emp.UserID == user.UserID
	isManager := managerEmp != nil && emp.ManagerID == managerEmp.ID
	if user.RoleName == "Employee" && !isSelf {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName == "Manager" && !isSelf && !isManager {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
		return
	}

	FilterEmployeeFields(emp, user, isSelf, isManager)
	api.Success(w, emp, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateEmployee(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != "HR" {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload Employee
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	id, err := h.Store.CreateEmployee(r.Context(), user.TenantID, payload)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "employee_create_failed", "failed to create employee", middleware.GetRequestID(r.Context()))
		return
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

	var payload Employee
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName != "HR" {
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

	var departments []Department
	for rows.Next() {
		var dep Department
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
	if user.RoleName != "HR" {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload Department
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
