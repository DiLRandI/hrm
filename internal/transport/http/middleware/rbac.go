package middleware

import (
	"context"
	"net/http"

	"hrm/internal/transport/http/api"
)

type PermissionStore interface {
	HasPermission(ctx context.Context, roleID, permission string) (bool, error)
}

func RequirePermission(permission string, store PermissionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := GetUser(r.Context())
			if !ok {
				api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", GetRequestID(r.Context()))
				return
			}

			allowed, err := store.HasPermission(r.Context(), user.RoleID, permission)
			if err != nil {
				api.Fail(w, http.StatusInternalServerError, "permission_error", "permission check failed", GetRequestID(r.Context()))
				return
			}
			if !allowed {
				api.Fail(w, http.StatusForbidden, "forbidden", "insufficient permissions", GetRequestID(r.Context()))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
