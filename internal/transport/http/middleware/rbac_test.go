package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"hrm/internal/domain/auth"
)

type stubPermissionStore struct {
	allowed map[string]bool
	errs    map[string]error
}

func (s stubPermissionStore) HasPermission(_ context.Context, _ string, permission string) (bool, error) {
	if err, ok := s.errs[permission]; ok {
		return false, err
	}
	return s.allowed[permission], nil
}

func withUser(req *http.Request) *http.Request {
	ctx := context.WithValue(req.Context(), ctxKeyUser, auth.UserContext{
		UserID:   "user-1",
		TenantID: "tenant-1",
		RoleID:   "role-1",
	})
	return req.WithContext(ctx)
}

func TestRequirePermission_AllowsSystemAdminOverride(t *testing.T) {
	store := stubPermissionStore{
		allowed: map[string]bool{
			auth.PermSystemAdmin: true,
		},
	}

	protected := RequirePermission(auth.PermEmployeesRead, store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := withUser(httptest.NewRequest(http.MethodGet, "/api/v1/employees", nil))
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rr.Code)
	}
}

func TestRequirePermission_DeniesWhenNoPermission(t *testing.T) {
	store := stubPermissionStore{
		allowed: map[string]bool{},
	}

	protected := RequirePermission(auth.PermEmployeesRead, store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := withUser(httptest.NewRequest(http.MethodGet, "/api/v1/employees", nil))
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestRequirePermission_ReturnsServerErrorWhenAdminCheckFails(t *testing.T) {
	store := stubPermissionStore{
		allowed: map[string]bool{},
		errs: map[string]error{
			auth.PermSystemAdmin: errors.New("db down"),
		},
	}

	protected := RequirePermission(auth.PermEmployeesRead, store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := withUser(httptest.NewRequest(http.MethodGet, "/api/v1/employees", nil))
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}
