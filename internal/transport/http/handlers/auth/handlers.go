package authhandler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
	Service *auth.Service
	Secret  string
	Crypto  *cryptoutil.Service
}

func NewHandler(service *auth.Service, secret string, crypto *cryptoutil.Service) *Handler {
	return &Handler{Service: service, Secret: secret, Crypto: crypto}
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

	userRow, err := h.Service.FindActiveUserByEmail(r.Context(), payload.Email, core.UserStatusActive)
	if err != nil {
		api.Fail(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials", requestctx.GetRequestID(r.Context()))
		return
	}

	if err := auth.CheckPassword(userRow.Password, payload.Password); err != nil {
		api.Fail(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials", requestctx.GetRequestID(r.Context()))
		return
	}

	if userRow.MFAEnabled {
		if payload.MFACode == "" {
			api.Fail(w, http.StatusUnauthorized, "mfa_required", "mfa code required", requestctx.GetRequestID(r.Context()))
			return
		}
		secret := ""
		if h.Crypto != nil && h.Crypto.Configured() {
			decoded, err := h.Crypto.DecryptString(userRow.MFASecretEn)
			if err != nil {
				api.Fail(w, http.StatusUnauthorized, "mfa_invalid", "invalid mfa configuration", requestctx.GetRequestID(r.Context()))
				return
			}
			secret = decoded
		} else {
			secret = string(userRow.MFASecretEn)
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
	if err := h.Service.CreateSession(r.Context(), userRow.ID, auth.HashToken(sessionID), sessionExpires); err != nil {
		api.Fail(w, http.StatusInternalServerError, "session_error", "failed to start session", requestctx.GetRequestID(r.Context()))
		return
	}

	token, err := auth.GenerateToken(h.Secret, auth.Claims{UserID: userRow.ID, TenantID: userRow.TenantID, RoleID: userRow.RoleID, RoleName: userRow.RoleName, SessionID: sessionID}, 8*time.Hour)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "token_error", "failed to issue token", requestctx.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.UpdateLastLogin(r.Context(), userRow.ID); err != nil {
		slog.Warn("update last_login failed", "userId", userRow.ID, "err", err)
	}

	api.Success(w, map[string]any{
		"token": token,
		"user":  map[string]string{"id": userRow.ID, "tenantId": userRow.TenantID, "roleId": userRow.RoleID, "role": userRow.RoleName},
	}, requestctx.GetRequestID(r.Context()))
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if user, ok := middleware.GetUser(r.Context()); ok && user.SessionID != "" {
		if err := h.Service.RevokeSession(r.Context(), user.UserID, auth.HashToken(user.SessionID)); err != nil {
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

	valid, err := h.Service.SessionValid(r.Context(), claims.UserID, auth.HashToken(claims.SessionID))
	if err != nil || !valid {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "session expired", requestctx.GetRequestID(r.Context()))
		return
	}

	newSessionID, err := generateToken()
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "token_error", "failed to rotate session", requestctx.GetRequestID(r.Context()))
		return
	}
	sessionExpires := time.Now().Add(8 * time.Hour)
	if err := h.Service.RotateSession(r.Context(), claims.UserID, auth.HashToken(claims.SessionID), auth.HashToken(newSessionID), sessionExpires); err != nil {
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
	if err := h.Service.UpdateMFASecret(r.Context(), user.UserID, encrypted); err != nil {
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

	secretEnc, err := h.Service.GetMFASecret(r.Context(), user.UserID)
	if err != nil {
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

	if err := h.Service.SetMFAEnabled(r.Context(), user.UserID, true); err != nil {
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
	secretEnc, err := h.Service.GetMFASecret(r.Context(), user.UserID)
	if err != nil {
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
	if err := h.Service.SetMFAEnabled(r.Context(), user.UserID, false); err != nil {
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

	userID, err := h.Service.UserIDByEmail(r.Context(), payload.Email)
	if err == nil {
		token, err := generateToken()
		if err != nil {
			slog.Warn("password reset token generation failed", "userId", userID, "err", err)
			api.Success(w, map[string]string{"status": "reset_requested"}, requestctx.GetRequestID(r.Context()))
			return
		}
		expires := time.Now().Add(2 * time.Hour)
		hashed := auth.HashToken(token)
		if err := h.Service.CreatePasswordReset(r.Context(), userID, hashed, expires); err != nil {
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

	userID, err := h.Service.PasswordResetUserID(r.Context(), auth.HashToken(payload.Token))
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_token", "invalid or expired token", requestctx.GetRequestID(r.Context()))
		return
	}

	hash, err := auth.HashPassword(payload.NewPassword)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "hash_error", "failed to update password", requestctx.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.UpdateUserPassword(r.Context(), userID, hash); err != nil {
		api.Fail(w, http.StatusInternalServerError, "update_failed", "failed to update password", requestctx.GetRequestID(r.Context()))
		return
	}
	if err := h.Service.MarkPasswordResetUsed(r.Context(), auth.HashToken(payload.Token)); err != nil {
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
