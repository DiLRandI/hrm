package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"hrm/internal/app/server"
	"hrm/internal/domain/auth"
	"hrm/internal/platform/config"
)

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error any             `json:"error"`
}

func TestHRLeaveAndPayrollJourney(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	cfg := config.Config{
		DatabaseURL:        dbURL,
		JWTSecret:          "test-secret",
		DataEncryptionKey:  "0123456789abcdef0123456789abcdef",
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
	}

	app, err := server.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to start app: %v", err)
	}
	defer app.Close()

	ts := httptest.NewServer(app.Router)
	defer ts.Close()

	client := ts.Client()
	token := login(t, client, ts.URL, cfg.SeedAdminEmail, cfg.SeedAdminPassword)

	employeeEmail := fmt.Sprintf("journey-%d@example.com", time.Now().UnixNano())
	employeeID := createEmployee(t, client, ts.URL, token, employeeEmail)

	leaveTypeID := createLeaveType(t, client, ts.URL, token)
	createLeavePolicy(t, client, ts.URL, token, leaveTypeID)
	requestID := createLeaveRequest(t, client, ts.URL, token, employeeID, leaveTypeID)

	status := approveLeaveRequest(t, client, ts.URL, token, requestID)
	if status != "approved" {
		t.Fatalf("expected leave approval status approved, got %s", status)
	}

	balances := listLeaveBalances(t, client, ts.URL, token, employeeID)
	if len(balances) == 0 {
		t.Fatal("expected leave balances to be updated")
	}

	scheduleID := createPayrollSchedule(t, client, ts.URL, token)
	periodID := createPayrollPeriod(t, client, ts.URL, token, scheduleID)

	runStatus := runPayroll(t, client, ts.URL, token, periodID)
	if runStatus != "reviewed" {
		t.Fatalf("expected payroll status reviewed, got %s", runStatus)
	}

	finalStatus := finalizePayroll(t, client, ts.URL, token, periodID)
	if finalStatus != "finalized" {
		t.Fatalf("expected payroll status finalized, got %s", finalStatus)
	}

	payslips := listPayslips(t, client, ts.URL, token, employeeID)
	if len(payslips) == 0 {
		t.Fatal("expected payslips to be generated")
	}
}

func TestManagerCannotAccessOtherEmployeeBalances(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	cfg := config.Config{
		DatabaseURL:        dbURL,
		JWTSecret:          "test-secret",
		DataEncryptionKey:  "0123456789abcdef0123456789abcdef",
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
	}

	app, err := server.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to start app: %v", err)
	}
	defer app.Close()

	ctx := context.Background()
	var tenantID string
	if err := app.DB.QueryRow(ctx, "SELECT id FROM tenants WHERE name = $1", cfg.SeedTenantName).Scan(&tenantID); err != nil {
		t.Fatalf("failed to load tenant: %v", err)
	}

	var managerRoleID string
	if err := app.DB.QueryRow(ctx, "SELECT id FROM roles WHERE tenant_id = $1 AND name = $2", tenantID, auth.RoleManager).Scan(&managerRoleID); err != nil {
		t.Fatalf("failed to load manager role: %v", err)
	}

	managerEmail := fmt.Sprintf("manager-%d@example.com", time.Now().UnixNano())
	managerPassword := "Manager123!"
	hash, err := auth.HashPassword(managerPassword)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	var managerUserID string
	if err := app.DB.QueryRow(ctx, `
    INSERT INTO users (tenant_id, email, password_hash, role_id)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, managerEmail, hash, managerRoleID).Scan(&managerUserID); err != nil {
		t.Fatalf("failed to create manager user: %v", err)
	}

	var managerEmployeeID string
	if err := app.DB.QueryRow(ctx, `
    INSERT INTO employees (tenant_id, user_id, first_name, last_name, email, status)
    VALUES ($1,$2,$3,$4,$5,$6)
    RETURNING id
  `, tenantID, managerUserID, "Manny", "Manager", managerEmail, "active").Scan(&managerEmployeeID); err != nil {
		t.Fatalf("failed to create manager employee: %v", err)
	}

	var otherEmployeeID string
	if err := app.DB.QueryRow(ctx, `
    INSERT INTO employees (tenant_id, first_name, last_name, email, status)
    VALUES ($1,$2,$3,$4,$5)
    RETURNING id
  `, tenantID, "Other", "Employee", fmt.Sprintf("other-%d@example.com", time.Now().UnixNano()), "active").Scan(&otherEmployeeID); err != nil {
		t.Fatalf("failed to create other employee: %v", err)
	}

	ts := httptest.NewServer(app.Router)
	defer ts.Close()

	token := login(t, ts.Client(), ts.URL, managerEmail, managerPassword)
	getJSONStatus(t, ts.Client(), ts.URL+"/api/v1/leave/balances?employeeId="+otherEmployeeID, token, http.StatusForbidden)

	_ = managerEmployeeID
}

func login(t *testing.T, client *http.Client, baseURL, email, password string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/auth/login", "", map[string]any{
		"email":    email,
		"password": password,
	})
	var payload map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	token, _ := payload["token"].(string)
	if token == "" {
		t.Fatal("expected token")
	}
	return token
}

func createEmployee(t *testing.T, client *http.Client, baseURL, token, email string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/employees", token, map[string]any{
		"firstName":      "Journey",
		"lastName":       "Tester",
		"email":          email,
		"status":         "active",
		"salary":         3500,
		"currency":       "USD",
		"employmentType": "full_time",
	})
	var payload map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode employee response: %v", err)
	}
	id, _ := payload["id"].(string)
	if id == "" {
		t.Fatal("expected employee id")
	}
	return id
}

func createLeaveType(t *testing.T, client *http.Client, baseURL, token string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/leave/types", token, map[string]any{
		"name":        "Annual",
		"code":        "ANL",
		"isPaid":      true,
		"requiresDoc": false,
	})
	var payload map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode leave type response: %v", err)
	}
	id, _ := payload["id"].(string)
	if id == "" {
		t.Fatal("expected leave type id")
	}
	return id
}

func createLeavePolicy(t *testing.T, client *http.Client, baseURL, token, leaveTypeID string) {
	t.Helper()
	postJSON(t, client, baseURL+"/api/v1/leave/policies", token, map[string]any{
		"leaveTypeId":        leaveTypeID,
		"accrualRate":        1.5,
		"accrualPeriod":      "monthly",
		"entitlement":        18,
		"carryOverLimit":     0,
		"allowNegative":      false,
		"requiresHrApproval": true,
	})
}

func createLeaveRequest(t *testing.T, client *http.Client, baseURL, token, employeeID, leaveTypeID string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/leave/requests", token, map[string]any{
		"employeeId":  employeeID,
		"leaveTypeId": leaveTypeID,
		"startDate":   "2026-01-10",
		"endDate":     "2026-01-12",
		"reason":      "Rest",
	})
	var payload map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode leave request response: %v", err)
	}
	id, _ := payload["id"].(string)
	if id == "" {
		t.Fatal("expected leave request id")
	}
	return id
}

func approveLeaveRequest(t *testing.T, client *http.Client, baseURL, token, requestID string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/leave/requests/"+requestID+"/approve", token, map[string]any{})
	var payload map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode leave approve response: %v", err)
	}
	status, _ := payload["status"].(string)
	return status
}

func listLeaveBalances(t *testing.T, client *http.Client, baseURL, token, employeeID string) []map[string]any {
	t.Helper()
	resp := getJSON(t, client, baseURL+"/api/v1/leave/balances?employeeId="+employeeID, token)
	var payload []map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode leave balances response: %v", err)
	}
	return payload
}

func createPayrollSchedule(t *testing.T, client *http.Client, baseURL, token string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/payroll/schedules", token, map[string]any{
		"name":      "Monthly",
		"frequency": "monthly",
		"payDay":    25,
	})
	var payload map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode schedule response: %v", err)
	}
	id, _ := payload["id"].(string)
	if id == "" {
		t.Fatal("expected schedule id")
	}
	return id
}

func createPayrollPeriod(t *testing.T, client *http.Client, baseURL, token, scheduleID string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/payroll/periods", token, map[string]any{
		"scheduleId": scheduleID,
		"startDate":  "2026-01-01",
		"endDate":    "2026-01-31",
	})
	var payload map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode payroll period response: %v", err)
	}
	id, _ := payload["id"].(string)
	if id == "" {
		t.Fatal("expected payroll period id")
	}
	return id
}

func runPayroll(t *testing.T, client *http.Client, baseURL, token, periodID string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/payroll/periods/"+periodID+"/run", token, map[string]any{})
	var payload map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode payroll run response: %v", err)
	}
	status, _ := payload["status"].(string)
	return status
}

func finalizePayroll(t *testing.T, client *http.Client, baseURL, token, periodID string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/payroll/periods/"+periodID+"/finalize", token, map[string]any{})
	var payload map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode payroll finalize response: %v", err)
	}
	status, _ := payload["status"].(string)
	return status
}

func listPayslips(t *testing.T, client *http.Client, baseURL, token, employeeID string) []map[string]any {
	t.Helper()
	resp := getJSON(t, client, baseURL+"/api/v1/payroll/payslips?employeeId="+employeeID, token)
	var payload []map[string]any
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode payslips response: %v", err)
	}
	return payload
}

func postJSON(t *testing.T, client *http.Client, url, token string, body any) envelope {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
		reader = bytes.NewBuffer(raw)
	}
	req, err := http.NewRequest(http.MethodPost, url, reader)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.StatusCode >= 400 {
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(raw))
	}
	return env
}

func getJSON(t *testing.T, client *http.Client, url, token string) envelope {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.StatusCode >= 400 {
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(raw))
	}
	return env
}

func getJSONStatus(t *testing.T, client *http.Client, url, token string, want int) envelope {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	if resp.StatusCode != want {
		t.Fatalf("expected status %d, got %d: %s", want, resp.StatusCode, string(raw))
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return env
}
