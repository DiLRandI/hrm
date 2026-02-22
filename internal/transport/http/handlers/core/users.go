package corehandler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type createUserPayload struct {
	Email    string         `json:"email"`
	Role     string         `json:"role"`
	Status   string         `json:"status"`
	Employee *core.Employee `json:"employee,omitempty"`
}

type updateUserRolePayload struct {
	Role string `json:"role"`
}

type updateUserStatusPayload struct {
	Status string `json:"status"`
}

func (h *Handler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if !canListUsers(user.RoleName) {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
		return
	}

	page := shared.ParsePagination(r, 100, 500)
	total, err := h.Service.CountUsers(r.Context(), user.TenantID)
	if err != nil {
		slog.Warn("user count failed", "err", err)
	}
	users, err := h.Service.ListUsers(r.Context(), user.TenantID, page.Limit, page.Offset)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "user_list_failed", "failed to list users", middleware.GetRequestID(r.Context()))
		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	api.Success(w, users, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload createUserPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "must be valid JSON"},
		})
		return
	}

	payload.Email = strings.TrimSpace(payload.Email)
	roleName := normalizeRoleName(payload.Role)
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status == "" {
		status = core.UserStatusActive
	}

	validator := shared.NewValidator()
	validator.Required("email", payload.Email, "is required")
	if roleName == "" {
		validator.Add("role", "must be one of: Employee, Manager, HRManager, HR, Admin, SystemAdmin")
	}
	validator.Enum("status", status, []string{core.UserStatusActive, core.UserStatusDisabled}, "must be one of: active, disabled")
	if requiresEmployeeProfile(roleName) && payload.Employee == nil {
		validator.Add("employee", "is required for Employee or Manager role onboarding")
	}
	if requiresEmployeeProfile(roleName) && payload.Employee != nil {
		validator.Required("employee.firstName", strings.TrimSpace(payload.Employee.FirstName), "is required")
		validator.Required("employee.lastName", strings.TrimSpace(payload.Employee.LastName), "is required")
	}
	if validator.Reject(w, middleware.GetRequestID(r.Context())) {
		return
	}

	if !canCreateRole(user.RoleName, roleName) {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed to create this role", middleware.GetRequestID(r.Context()))
		return
	}

	tempPassword, err := generateTempPassword()
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "password_generate_failed", "failed to generate temporary password", middleware.GetRequestID(r.Context()))
		return
	}

	var employeePayload *core.Employee
	if payload.Employee != nil {
		normalized := *payload.Employee
		if strings.TrimSpace(normalized.Email) == "" {
			normalized.Email = payload.Email
		}
		if strings.TrimSpace(normalized.Status) == "" {
			normalized.Status = core.EmployeeStatusActive
		}
		employeePayload = &normalized
	}

	userID, employeeID, err := h.Service.CreateUserWithEmployee(r.Context(), user.TenantID, payload.Email, tempPassword, roleName, status, employeePayload)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			api.Fail(w, http.StatusConflict, "user_exists", "user email already exists", middleware.GetRequestID(r.Context()))
			return
		}
		api.Fail(w, http.StatusInternalServerError, "user_create_failed", "failed to create user", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "core.user.create", "user", userID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{
		"role":       roleName,
		"status":     status,
		"employeeId": employeeID,
	}); err != nil {
		slog.Warn("audit core.user.create failed", "err", err)
	}

	api.Created(w, map[string]any{
		"id":           userID,
		"role":         roleName,
		"status":       status,
		"employeeId":   employeeID,
		"tempPassword": tempPassword,
	}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	userID := chi.URLParam(r, "userID")
	if userID == user.UserID {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "self role change is not allowed", middleware.GetRequestID(r.Context()))
		return
	}

	var payload updateUserRolePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "must be valid JSON"},
		})
		return
	}

	roleName := normalizeRoleName(payload.Role)
	if roleName == "" {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "role", Reason: "must be one of: Employee, Manager, HRManager, HR, Admin, SystemAdmin"},
		})
		return
	}

	target, err := h.Service.GetUser(r.Context(), user.TenantID, userID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "user not found", middleware.GetRequestID(r.Context()))
		return
	}
	if !canManageRole(user.RoleName, target.RoleName) || !canCreateRole(user.RoleName, roleName) {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed to change this role", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.UpdateUserRole(r.Context(), user.TenantID, userID, roleName); err != nil {
		api.Fail(w, http.StatusInternalServerError, "user_role_update_failed", "failed to update user role", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "core.user.role.update", "user", userID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), map[string]any{
		"role": target.RoleName,
	}, map[string]any{
		"role": roleName,
	}); err != nil {
		slog.Warn("audit core.user.role.update failed", "err", err)
	}

	api.Success(w, map[string]any{"id": userID, "role": roleName}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleUpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	userID := chi.URLParam(r, "userID")
	if userID == user.UserID {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "self status change is not allowed", middleware.GetRequestID(r.Context()))
		return
	}

	var payload updateUserStatusPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "payload", Reason: "must be valid JSON"},
		})
		return
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status == "" {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "status", Reason: "is required"},
		})
		return
	}
	if status != core.UserStatusActive && status != core.UserStatusDisabled {
		shared.FailValidation(w, middleware.GetRequestID(r.Context()), []shared.ValidationIssue{
			{Field: "status", Reason: "must be one of: active, disabled"},
		})
		return
	}

	target, err := h.Service.GetUser(r.Context(), user.TenantID, userID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "user not found", middleware.GetRequestID(r.Context()))
		return
	}
	if !canManageRole(user.RoleName, target.RoleName) {
		api.Fail(w, http.StatusForbidden, "forbidden", "not allowed to update this user", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.UpdateUserStatus(r.Context(), user.TenantID, userID, status); err != nil {
		api.Fail(w, http.StatusInternalServerError, "user_status_update_failed", "failed to update user status", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "core.user.status.update", "user", userID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), map[string]any{
		"status": target.Status,
	}, map[string]any{
		"status": status,
	}); err != nil {
		slog.Warn("audit core.user.status.update failed", "err", err)
	}

	api.Success(w, map[string]any{"id": userID, "status": status}, middleware.GetRequestID(r.Context()))
}

func normalizeRoleName(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case strings.ToLower(auth.RoleEmployee):
		return auth.RoleEmployee
	case strings.ToLower(auth.RoleManager):
		return auth.RoleManager
	case strings.ToLower(auth.RoleHRManager), "hr_manager", "hr-manager":
		return auth.RoleHRManager
	case strings.ToLower(auth.RoleHR):
		return auth.RoleHR
	case strings.ToLower(auth.RoleAdmin):
		return auth.RoleAdmin
	case strings.ToLower(auth.RoleSystemAdmin), "system_admin", "system-admin":
		return auth.RoleSystemAdmin
	default:
		return ""
	}
}

func canListUsers(actorRole string) bool {
	return actorRole == auth.RoleSystemAdmin || actorRole == auth.RoleAdmin || actorRole == auth.RoleHRManager || actorRole == auth.RoleHR
}

func canCreateRole(actorRole, targetRole string) bool {
	switch actorRole {
	case auth.RoleSystemAdmin:
		return targetRole == auth.RoleAdmin || targetRole == auth.RoleHR || targetRole == auth.RoleHRManager || targetRole == auth.RoleManager
	case auth.RoleAdmin:
		return targetRole == auth.RoleHRManager || targetRole == auth.RoleHR || targetRole == auth.RoleManager
	case auth.RoleHRManager:
		return targetRole == auth.RoleHR || targetRole == auth.RoleEmployee
	case auth.RoleHR:
		return targetRole == auth.RoleEmployee
	default:
		return false
	}
}

func canManageRole(actorRole, targetRole string) bool {
	switch actorRole {
	case auth.RoleSystemAdmin:
		return targetRole == auth.RoleAdmin || targetRole == auth.RoleHR || targetRole == auth.RoleHRManager || targetRole == auth.RoleManager || targetRole == auth.RoleEmployee
	case auth.RoleAdmin:
		return targetRole == auth.RoleHR || targetRole == auth.RoleHRManager || targetRole == auth.RoleManager || targetRole == auth.RoleEmployee
	case auth.RoleHRManager:
		return targetRole == auth.RoleHR || targetRole == auth.RoleEmployee
	case auth.RoleHR:
		return targetRole == auth.RoleEmployee
	default:
		return false
	}
}

func requiresEmployeeProfile(roleName string) bool {
	return roleName == auth.RoleEmployee || roleName == auth.RoleManager
}
