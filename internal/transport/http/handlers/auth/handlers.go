package authhandler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
	"hrm/internal/platform/requestctx"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
)

type Handler struct {
	DB     *pgxpool.Pool
	Secret string
}

func NewHandler(db *pgxpool.Pool, secret string) *Handler {
	return &Handler{DB: db, Secret: secret}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type resetRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

func (h *Handler) RegisterRoutes(r *http.ServeMux) {}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var payload loginRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", requestctx.GetRequestID(r.Context()))
		return
	}

	var id, tenantID, roleID, roleName, hash string
	err := h.DB.QueryRow(r.Context(), `
    SELECT u.id, u.tenant_id, u.role_id, r.name, u.password_hash
    FROM users u
    JOIN roles r ON u.role_id = r.id
    WHERE u.email = $1 AND u.status = $2
  `, payload.Email, core.UserStatusActive).Scan(&id, &tenantID, &roleID, &roleName, &hash)
	if err != nil {
		api.Fail(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials", requestctx.GetRequestID(r.Context()))
		return
	}

	if err := auth.CheckPassword(hash, payload.Password); err != nil {
		api.Fail(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials", requestctx.GetRequestID(r.Context()))
		return
	}

	sessionID, err := generateToken()
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "token_error", "failed to issue token", requestctx.GetRequestID(r.Context()))
		return
	}
	sessionExpires := time.Now().Add(8 * time.Hour)
	if _, err := h.DB.Exec(r.Context(), `
    INSERT INTO sessions (user_id, refresh_token, expires_at)
    VALUES ($1,$2,$3)
  `, id, auth.HashToken(sessionID), sessionExpires); err != nil {
		api.Fail(w, http.StatusInternalServerError, "session_error", "failed to start session", requestctx.GetRequestID(r.Context()))
		return
	}

	token, err := auth.GenerateToken(h.Secret, auth.Claims{UserID: id, TenantID: tenantID, RoleID: roleID, RoleName: roleName, SessionID: sessionID}, 8*time.Hour)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "token_error", "failed to issue token", requestctx.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), "UPDATE users SET last_login = now() WHERE id = $1", id); err != nil {
		slog.Warn("update last_login failed", "userId", id, "err", err)
	}

	api.Success(w, map[string]any{
		"token": token,
		"user":  map[string]string{"id": id, "tenantId": tenantID, "roleId": roleID, "role": roleName},
	}, requestctx.GetRequestID(r.Context()))
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if user, ok := middleware.GetUser(r.Context()); ok && user.SessionID != "" {
		if _, err := h.DB.Exec(r.Context(), "DELETE FROM sessions WHERE user_id = $1 AND refresh_token = $2", user.UserID, auth.HashToken(user.SessionID)); err != nil {
			slog.Warn("logout session delete failed", "userId", user.UserID, "err", err)
		}
	}
	api.Success(w, map[string]string{"status": "logged_out"}, requestctx.GetRequestID(r.Context()))
}

func (h *Handler) HandleRequestReset(w http.ResponseWriter, r *http.Request) {
	var payload resetRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", requestctx.GetRequestID(r.Context()))
		return
	}

	var userID string
	err := h.DB.QueryRow(r.Context(), "SELECT id FROM users WHERE email = $1", payload.Email).Scan(&userID)
	if err == nil {
		token, err := generateToken()
		if err != nil {
			slog.Warn("password reset token generation failed", "userId", userID, "err", err)
			api.Success(w, map[string]string{"status": "reset_requested"}, requestctx.GetRequestID(r.Context()))
			return
		}
		expires := time.Now().Add(2 * time.Hour)
		hashed := auth.HashToken(token)
		if _, err := h.DB.Exec(r.Context(), "INSERT INTO password_resets (user_id, token, expires_at) VALUES ($1, $2, $3)", userID, hashed, expires); err != nil {
			slog.Warn("password reset insert failed", "userId", userID, "err", err)
		}
	}

	api.Success(w, map[string]string{"status": "reset_requested"}, requestctx.GetRequestID(r.Context()))
}

func (h *Handler) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	var payload resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", requestctx.GetRequestID(r.Context()))
		return
	}

	var userID string
	err := h.DB.QueryRow(r.Context(), `
    SELECT user_id
    FROM password_resets
    WHERE token = $1 AND expires_at > now() AND used_at IS NULL
  `, auth.HashToken(payload.Token)).Scan(&userID)
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_token", "invalid or expired token", requestctx.GetRequestID(r.Context()))
		return
	}

	hash, err := auth.HashPassword(payload.NewPassword)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "hash_error", "failed to update password", requestctx.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), "UPDATE users SET password_hash = $1 WHERE id = $2", hash, userID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "update_failed", "failed to update password", requestctx.GetRequestID(r.Context()))
		return
	}
	if _, err := h.DB.Exec(r.Context(), "UPDATE password_resets SET used_at = now() WHERE token = $1", payload.Token); err != nil {
		slog.Warn("password reset mark used failed", "err", err)
	}

	api.Success(w, map[string]string{"status": "password_reset"}, requestctx.GetRequestID(r.Context()))
}

func generateToken() (string, error) {
	buff := make([]byte, 32)
	if _, err := rand.Read(buff); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buff), nil
}
