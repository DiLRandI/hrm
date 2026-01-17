package authhandler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
	cryptoutil "hrm/internal/platform/crypto"
	"hrm/internal/platform/requestctx"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
)

type Handler struct {
	DB     *pgxpool.Pool
	Secret string
	Crypto *cryptoutil.Service
}

func NewHandler(db *pgxpool.Pool, secret string, crypto *cryptoutil.Service) *Handler {
	return &Handler{DB: db, Secret: secret, Crypto: crypto}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	MFACode  string `json:"mfaCode"`
}

type resetRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type mfaCodeRequest struct {
	Code string `json:"code"`
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var payload loginRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", requestctx.GetRequestID(r.Context()))
		return
	}

	var id, tenantID, roleID, roleName, hash string
	var mfaEnabled bool
	var mfaSecretEnc []byte
	err := h.DB.QueryRow(r.Context(), `
    SELECT u.id, u.tenant_id, u.role_id, r.name, u.password_hash, u.mfa_enabled, u.mfa_secret_enc
    FROM users u
    JOIN roles r ON u.role_id = r.id
    WHERE u.email = $1 AND u.status = $2
  `, payload.Email, core.UserStatusActive).Scan(&id, &tenantID, &roleID, &roleName, &hash, &mfaEnabled, &mfaSecretEnc)
	if err != nil {
		api.Fail(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials", requestctx.GetRequestID(r.Context()))
		return
	}

	if err := auth.CheckPassword(hash, payload.Password); err != nil {
		api.Fail(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials", requestctx.GetRequestID(r.Context()))
		return
	}

	if mfaEnabled {
		if payload.MFACode == "" {
			api.Fail(w, http.StatusUnauthorized, "mfa_required", "mfa code required", requestctx.GetRequestID(r.Context()))
			return
		}
		secret := ""
		if h.Crypto != nil && h.Crypto.Configured() {
			decoded, err := h.Crypto.DecryptString(mfaSecretEnc)
			if err != nil {
				api.Fail(w, http.StatusUnauthorized, "mfa_invalid", "invalid mfa configuration", requestctx.GetRequestID(r.Context()))
				return
			}
			secret = decoded
		} else {
			secret = string(mfaSecretEnc)
		}
		if secret == "" || !totp.Validate(payload.MFACode, secret) {
			api.Fail(w, http.StatusUnauthorized, "mfa_invalid", "invalid mfa code", requestctx.GetRequestID(r.Context()))
			return
		}
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
		if _, err := h.DB.Exec(r.Context(), "UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND refresh_token = $2", user.UserID, auth.HashToken(user.SessionID)); err != nil {
			slog.Warn("logout session revoke failed", "userId", user.UserID, "err", err)
		}
	}
	api.Success(w, map[string]string{"status": "logged_out"}, requestctx.GetRequestID(r.Context()))
}

func (h *Handler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", requestctx.GetRequestID(r.Context()))
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", requestctx.GetRequestID(r.Context()))
		return
	}
	claims, err := auth.ParseToken(h.Secret, parts[1])
	if err != nil {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", requestctx.GetRequestID(r.Context()))
		return
	}

	var count int
	if err := h.DB.QueryRow(r.Context(), `
    SELECT COUNT(1)
    FROM sessions
    WHERE user_id = $1 AND refresh_token = $2 AND expires_at > now() AND revoked_at IS NULL
  `, claims.UserID, auth.HashToken(claims.SessionID)).Scan(&count); err != nil || count == 0 {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "session expired", requestctx.GetRequestID(r.Context()))
		return
	}

	newSessionID, err := generateToken()
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "token_error", "failed to rotate session", requestctx.GetRequestID(r.Context()))
		return
	}
	sessionExpires := time.Now().Add(8 * time.Hour)
	if _, err := h.DB.Exec(r.Context(), `
    UPDATE sessions
    SET refresh_token = $1, expires_at = $2, rotated_at = now()
    WHERE user_id = $3 AND refresh_token = $4
  `, auth.HashToken(newSessionID), sessionExpires, claims.UserID, auth.HashToken(claims.SessionID)); err != nil {
		api.Fail(w, http.StatusInternalServerError, "session_error", "failed to rotate session", requestctx.GetRequestID(r.Context()))
		return
	}

	token, err := auth.GenerateToken(h.Secret, auth.Claims{
		UserID:    claims.UserID,
		TenantID:  claims.TenantID,
		RoleID:    claims.RoleID,
		RoleName:  claims.RoleName,
		SessionID: newSessionID,
	}, 8*time.Hour)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "token_error", "failed to issue token", requestctx.GetRequestID(r.Context()))
		return
	}
	api.Success(w, map[string]any{"token": token}, requestctx.GetRequestID(r.Context()))
}

func (h *Handler) HandleMFASetup(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", requestctx.GetRequestID(r.Context()))
		return
	}
	if h.Crypto == nil || !h.Crypto.Configured() {
		api.Fail(w, http.StatusBadRequest, "mfa_unavailable", "mfa requires encryption key", requestctx.GetRequestID(r.Context()))
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "PulseHR",
		AccountName: user.UserID,
		Period:      30,
		Digits:      otp.DigitsSix,
	})
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "mfa_setup_failed", "failed to generate mfa secret", requestctx.GetRequestID(r.Context()))
		return
	}
	secret := key.Secret()
	encrypted, err := h.Crypto.EncryptString(secret)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "mfa_setup_failed", "failed to store mfa secret", requestctx.GetRequestID(r.Context()))
		return
	}
	if _, err := h.DB.Exec(r.Context(), `
    UPDATE users SET mfa_secret_enc = $1, mfa_enabled = false WHERE id = $2
  `, encrypted, user.UserID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "mfa_setup_failed", "failed to store mfa secret", requestctx.GetRequestID(r.Context()))
		return
	}

	api.Success(w, map[string]string{"secret": secret, "otpauthUrl": key.URL()}, requestctx.GetRequestID(r.Context()))
}

func (h *Handler) HandleMFAEnable(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", requestctx.GetRequestID(r.Context()))
		return
	}
	if h.Crypto == nil || !h.Crypto.Configured() {
		api.Fail(w, http.StatusBadRequest, "mfa_unavailable", "mfa requires encryption key", requestctx.GetRequestID(r.Context()))
		return
	}

	var payload mfaCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", requestctx.GetRequestID(r.Context()))
		return
	}

	var secretEnc []byte
	if err := h.DB.QueryRow(r.Context(), "SELECT mfa_secret_enc FROM users WHERE id = $1", user.UserID).Scan(&secretEnc); err != nil {
		api.Fail(w, http.StatusBadRequest, "mfa_missing", "mfa setup required", requestctx.GetRequestID(r.Context()))
		return
	}
	secret, err := h.Crypto.DecryptString(secretEnc)
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "mfa_invalid", "invalid mfa secret", requestctx.GetRequestID(r.Context()))
		return
	}
	if !totp.Validate(payload.Code, secret) {
		api.Fail(w, http.StatusBadRequest, "mfa_invalid", "invalid mfa code", requestctx.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), "UPDATE users SET mfa_enabled = true WHERE id = $1", user.UserID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "mfa_enable_failed", "failed to enable mfa", requestctx.GetRequestID(r.Context()))
		return
	}
	api.Success(w, map[string]string{"status": "enabled"}, requestctx.GetRequestID(r.Context()))
}

func (h *Handler) HandleMFADisable(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", requestctx.GetRequestID(r.Context()))
		return
	}
	if h.Crypto == nil || !h.Crypto.Configured() {
		api.Fail(w, http.StatusBadRequest, "mfa_unavailable", "mfa requires encryption key", requestctx.GetRequestID(r.Context()))
		return
	}
	var payload mfaCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", requestctx.GetRequestID(r.Context()))
		return
	}
	var secretEnc []byte
	if err := h.DB.QueryRow(r.Context(), "SELECT mfa_secret_enc FROM users WHERE id = $1", user.UserID).Scan(&secretEnc); err != nil {
		api.Fail(w, http.StatusBadRequest, "mfa_missing", "mfa setup required", requestctx.GetRequestID(r.Context()))
		return
	}
	secret, err := h.Crypto.DecryptString(secretEnc)
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "mfa_invalid", "invalid mfa secret", requestctx.GetRequestID(r.Context()))
		return
	}
	if !totp.Validate(payload.Code, secret) {
		api.Fail(w, http.StatusBadRequest, "mfa_invalid", "invalid mfa code", requestctx.GetRequestID(r.Context()))
		return
	}
	if _, err := h.DB.Exec(r.Context(), "UPDATE users SET mfa_enabled = false WHERE id = $1", user.UserID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "mfa_disable_failed", "failed to disable mfa", requestctx.GetRequestID(r.Context()))
		return
	}
	api.Success(w, map[string]string{"status": "disabled"}, requestctx.GetRequestID(r.Context()))
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
	if _, err := h.DB.Exec(r.Context(), "UPDATE password_resets SET used_at = now() WHERE token = $1", auth.HashToken(payload.Token)); err != nil {
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
