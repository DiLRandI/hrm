package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hrm/internal/domain/auth"
)

func TestAuthMiddlewareSetsUser(t *testing.T) {
	secret := "test-secret"
	token, err := auth.GenerateToken(secret, auth.Claims{UserID: "u1", TenantID: "t1", RoleID: "r1", RoleName: auth.RoleHR}, time.Hour)
	if err != nil {
		t.Fatalf("token error: %v", err)
	}

	handler := Auth(secret, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUser(r.Context())
		if !ok {
			t.Fatal("expected user in context")
		}
		if user.UserID != "u1" || user.RoleName != auth.RoleHR {
			t.Fatalf("unexpected user: %+v", user)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestAuthMiddlewareMissingToken(t *testing.T) {
	handler := Auth("secret", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := GetUser(r.Context()); ok {
			t.Fatal("did not expect user in context")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}
