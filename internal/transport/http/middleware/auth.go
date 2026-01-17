package middleware

import (
	"context"
	"net/http"
	"strings"

	"hrm/internal/domain/auth"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Auth(secret string, pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := auth.ParseToken(secret, parts[1])
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if pool != nil {
				if claims.SessionID == "" {
					next.ServeHTTP(w, r)
					return
				}
				var count int
				err := pool.QueryRow(r.Context(), `
          SELECT COUNT(1)
          FROM sessions
          WHERE user_id = $1 AND refresh_token = $2 AND expires_at > now()
        `, claims.UserID, auth.HashToken(claims.SessionID)).Scan(&count)
				if err != nil || count == 0 {
					next.ServeHTTP(w, r)
					return
				}
			}

			ctx := context.WithValue(r.Context(), ctxKeyUser, auth.UserContext{
				UserID:    claims.UserID,
				TenantID:  claims.TenantID,
				RoleID:    claims.RoleID,
				RoleName:  claims.RoleName,
				SessionID: claims.SessionID,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUser(ctx context.Context) (auth.UserContext, bool) {
	user, ok := ctx.Value(ctxKeyUser).(auth.UserContext)
	return user, ok
}
