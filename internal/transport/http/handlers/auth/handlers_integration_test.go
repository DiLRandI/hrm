package authhandler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"hrm/internal/app/server"
	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/platform/config"
	authhandler "hrm/internal/transport/http/handlers/auth"
)

type captureMailer struct {
	mu       sync.Mutex
	messages []emailMessage
}

type emailMessage struct {
	from    string
	to      string
	subject string
	body    string
}

func (m *captureMailer) Send(_ context.Context, from, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, emailMessage{
		from:    from,
		to:      to,
		subject: subject,
		body:    body,
	})
	return nil
}

func (m *captureMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *captureMailer) last() (emailMessage, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) == 0 {
		return emailMessage{}, false
	}
	return m.messages[len(m.messages)-1], true
}

type responseEnvelope struct {
	Success bool           `json:"success"`
	Data    map[string]any `json:"data"`
	Error   *responseError `json:"error"`
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func TestPasswordResetRequestDeliveryAndResetFlow(t *testing.T) {
	h, app, mailer, _, email, _ := newResetTestHarness(t)
	defer app.Close()

	status, env := postHandlerJSON(t, h.HandleRequestReset, "/api/v1/auth/request-reset", map[string]any{
		"email": email,
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 for request reset, got %d", status)
	}
	if !env.Success {
		t.Fatalf("expected request reset success envelope, got %+v", env)
	}
	if got := env.Data["status"]; got != "reset_requested" {
		t.Fatalf("expected reset_requested status, got %v", got)
	}
	if mailer.count() != 1 {
		t.Fatalf("expected one reset email, got %d", mailer.count())
	}

	message, ok := mailer.last()
	if !ok {
		t.Fatal("expected sent reset email")
	}
	token := extractResetToken(t, message.body)
	if token == "" {
		t.Fatal("expected reset token in email body")
	}

	var rawTokenCount int
	if err := app.DB.QueryRow(context.Background(), "SELECT COUNT(1) FROM password_resets WHERE token = $1", token).Scan(&rawTokenCount); err != nil {
		t.Fatalf("failed to count raw tokens: %v", err)
	}
	if rawTokenCount != 0 {
		t.Fatalf("expected raw token not stored, found %d rows", rawTokenCount)
	}

	var hashedTokenCount int
	if err := app.DB.QueryRow(context.Background(), "SELECT COUNT(1) FROM password_resets WHERE token = $1", auth.HashToken(token)).Scan(&hashedTokenCount); err != nil {
		t.Fatalf("failed to count hashed tokens: %v", err)
	}
	if hashedTokenCount != 1 {
		t.Fatalf("expected hashed token stored once, found %d rows", hashedTokenCount)
	}

	newPassword := "ResetStrong123"
	status, env = postHandlerJSON(t, h.HandleResetPassword, "/api/v1/auth/reset", map[string]any{
		"token":       token,
		"newPassword": newPassword,
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 for reset password, got %d", status)
	}
	if !env.Success {
		t.Fatalf("expected reset password success envelope, got %+v", env)
	}
	if got := env.Data["status"]; got != "password_reset" {
		t.Fatalf("expected password_reset status, got %v", got)
	}

	userRow, err := h.Service.FindActiveUserByEmail(context.Background(), email, "active")
	if err != nil {
		t.Fatalf("failed to load updated user: %v", err)
	}
	if err := auth.CheckPassword(userRow.Password, newPassword); err != nil {
		t.Fatalf("expected password to be updated to new value: %v", err)
	}

	var usedCount int
	if err := app.DB.QueryRow(context.Background(), "SELECT COUNT(1) FROM password_resets WHERE token = $1 AND used_at IS NOT NULL", auth.HashToken(token)).Scan(&usedCount); err != nil {
		t.Fatalf("failed to check used token state: %v", err)
	}
	if usedCount != 1 {
		t.Fatalf("expected used token mark, found %d rows", usedCount)
	}

	status, env = postHandlerJSON(t, h.HandleResetPassword, "/api/v1/auth/reset", map[string]any{
		"token":       token,
		"newPassword": "AnotherStrong123",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for reused token, got %d", status)
	}
	if env.Error == nil || env.Error.Code != "invalid_token" {
		t.Fatalf("expected invalid_token for reused token, got %+v", env.Error)
	}
}

func TestPasswordResetInvalidTokenRejected(t *testing.T) {
	h, app, _, _, _, _ := newResetTestHarness(t)
	defer app.Close()

	status, env := postHandlerJSON(t, h.HandleResetPassword, "/api/v1/auth/reset", map[string]any{
		"token":       "not-a-real-token",
		"newPassword": "StrongReset123",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid token, got %d", status)
	}
	if env.Error == nil || env.Error.Code != "invalid_token" {
		t.Fatalf("expected invalid_token for invalid token, got %+v", env.Error)
	}
}

func TestPasswordResetUnknownEmailReturnsGenericSuccess(t *testing.T) {
	h, app, mailer, _, _, _ := newResetTestHarness(t)
	defer app.Close()

	status, env := postHandlerJSON(t, h.HandleRequestReset, "/api/v1/auth/request-reset", map[string]any{
		"email": fmt.Sprintf("missing-%d@example.com", time.Now().UnixNano()),
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 for unknown-email request reset, got %d", status)
	}
	if !env.Success {
		t.Fatalf("expected generic success envelope for unknown email, got %+v", env)
	}
	if got := env.Data["status"]; got != "reset_requested" {
		t.Fatalf("expected reset_requested status for unknown email, got %v", got)
	}
	if mailer.count() != 0 {
		t.Fatalf("expected no email delivery for unknown account, got %d message(s)", mailer.count())
	}
}

func TestPasswordResetExpiredTokenRejected(t *testing.T) {
	h, app, _, userID, _, _ := newResetTestHarness(t)
	defer app.Close()

	expiredToken := fmt.Sprintf("expired-%d-token", time.Now().UnixNano())
	if err := h.Service.CreatePasswordReset(context.Background(), userID, auth.HashToken(expiredToken), time.Now().Add(-1*time.Hour)); err != nil {
		t.Fatalf("failed to seed expired reset token: %v", err)
	}

	status, env := postHandlerJSON(t, h.HandleResetPassword, "/api/v1/auth/reset", map[string]any{
		"token":       expiredToken,
		"newPassword": "ExpiredReset123",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired token, got %d", status)
	}
	if env.Error == nil || env.Error.Code != "invalid_token" {
		t.Fatalf("expected invalid_token for expired token, got %+v", env.Error)
	}
}

func newResetTestHarness(t *testing.T) (*authhandler.Handler, *server.App, *captureMailer, string, string, string) {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if strings.TrimSpace(dbURL) == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	cfg := config.Config{
		DatabaseURL:        dbURL,
		JWTSecret:          "test-secret",
		DataEncryptionKey:  "0123456789abcdef0123456789abcdef",
		FrontendBaseURL:    "https://hr.example.com/app",
		FrontendDir:        "frontend/dist",
		Environment:        "test",
		SeedTenantName:     "Test Tenant",
		SeedAdminEmail:     "admin@test.local",
		SeedAdminPassword:  "ChangeMe123!",
		EmailFrom:          "no-reply@test.local",
		RunMigrations:      true,
		RunSeed:            true,
		MaxBodyBytes:       1048576,
		RateLimitPerMinute: 1000,
		PasswordResetTTL:   2 * time.Hour,
	}

	app, err := server.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to start app: %v", err)
	}

	userID, email, initialPassword := createResetTestUser(t, app, cfg.SeedTenantName)
	mailer := &captureMailer{}
	service := auth.NewService(auth.NewStore(app.DB))
	handler := authhandler.NewHandler(service, cfg.JWTSecret, nil, mailer, cfg.EmailFrom, cfg.FrontendBaseURL, cfg.PasswordResetTTL, audit.New(app.DB))
	return handler, app, mailer, userID, email, initialPassword
}

func createResetTestUser(t *testing.T, app *server.App, tenantName string) (string, string, string) {
	t.Helper()
	ctx := context.Background()

	var tenantID string
	if err := app.DB.QueryRow(ctx, "SELECT id FROM tenants WHERE name = $1", tenantName).Scan(&tenantID); err != nil {
		t.Fatalf("failed to load tenant: %v", err)
	}

	var roleID string
	if err := app.DB.QueryRow(ctx, "SELECT id FROM roles WHERE tenant_id = $1 AND name = $2", tenantID, auth.RoleEmployee).Scan(&roleID); err != nil {
		t.Fatalf("failed to load employee role: %v", err)
	}

	initialPassword := "InitialReset123"
	passwordHash, err := auth.HashPassword(initialPassword)
	if err != nil {
		t.Fatalf("failed to hash initial password: %v", err)
	}

	email := fmt.Sprintf("reset-flow-%d@example.com", time.Now().UnixNano())
	var userID string
	if err := app.DB.QueryRow(ctx, `
    INSERT INTO users (tenant_id, email, password_hash, role_id)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, email, passwordHash, roleID).Scan(&userID); err != nil {
		t.Fatalf("failed to create reset test user: %v", err)
	}

	return userID, email, initialPassword
}

func postHandlerJSON(t *testing.T, handlerFunc http.HandlerFunc, path string, body any) (int, responseEnvelope) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.10:4321"
	recorder := httptest.NewRecorder()
	handlerFunc(recorder, req)

	var env responseEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response body %q: %v", recorder.Body.String(), err)
	}
	return recorder.Code, env
}

func extractResetToken(t *testing.T, body string) string {
	t.Helper()
	linkPattern := regexp.MustCompile(`https?://[^\s]+`)
	link := linkPattern.FindString(body)
	if link == "" {
		t.Fatalf("expected reset link in email body, got %q", body)
	}

	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("failed to parse reset link %q: %v", link, err)
	}
	token := parsed.Query().Get("token")
	if token == "" {
		t.Fatalf("expected token query param in reset link %q", link)
	}
	return token
}
