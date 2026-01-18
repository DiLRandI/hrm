package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"hrm/internal/app/server"
	"hrm/internal/domain/auth"
	"hrm/internal/platform/config"
)

func TestPerformanceReviewJourney(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	cfg := testConfig(dbURL)
	app, err := server.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to start app: %v", err)
	}
	defer app.Close()

	ts := httptest.NewServer(app.Router)
	defer ts.Close()

	client := ts.Client()
	adminToken := login(t, client, ts.URL, cfg.SeedAdminEmail, cfg.SeedAdminPassword)

	tenantID := getTenantID(t, app, cfg.SeedTenantName)

	managerEmail := fmt.Sprintf("manager-perf-%d@example.com", time.Now().UnixNano())
	managerPassword := "Manager123!"
	managerUserID := createUserWithRole(t, app, tenantID, auth.RoleManager, managerEmail, managerPassword)
	managerEmployeeID := createEmployeeWithUser(t, client, ts.URL, adminToken, managerUserID, "", managerEmail)

	employeeEmail := fmt.Sprintf("employee-perf-%d@example.com", time.Now().UnixNano())
	employeePassword := "Employee123!"
	employeeUserID := createUserWithRole(t, app, tenantID, auth.RoleEmployee, employeeEmail, employeePassword)
	employeeID := createEmployeeWithUser(t, client, ts.URL, adminToken, employeeUserID, managerEmployeeID, employeeEmail)

	templateID := createReviewTemplate(t, client, ts.URL, adminToken)
	cycleID := createReviewCycle(t, client, ts.URL, adminToken, templateID)

	employeeToken := login(t, client, ts.URL, employeeEmail, employeePassword)
	employeeTasks := listReviewTasks(t, client, ts.URL, employeeToken)
	if len(employeeTasks) == 0 {
		t.Fatal("expected employee review tasks")
	}
	taskID := employeeTasks[0]["id"].(string)
	submitReviewResponse(t, client, ts.URL, employeeToken, taskID, "self")

	managerToken := login(t, client, ts.URL, managerEmail, managerPassword)
	managerTasks := listReviewTasks(t, client, ts.URL, managerToken)
	managerTaskID := findTaskForEmployee(managerTasks, employeeID)
	if managerTaskID == "" {
		t.Fatal("expected manager task for employee")
	}
	submitReviewResponse(t, client, ts.URL, managerToken, managerTaskID, "manager")

	hrTasks := listReviewTasks(t, client, ts.URL, adminToken)
	hrTaskID := findTaskForEmployee(hrTasks, employeeID)
	if hrTaskID == "" {
		t.Fatal("expected HR task for employee")
	}
	submitReviewResponse(t, client, ts.URL, adminToken, hrTaskID, "hr")

	status := finalizeReviewCycle(t, client, ts.URL, adminToken, cycleID)
	if status != "closed" {
		t.Fatalf("expected review cycle closed, got %s", status)
	}
}

func TestGDPRRetentionAndDSARJourney(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	cfg := testConfig(dbURL)
	app, err := server.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to start app: %v", err)
	}
	defer app.Close()

	ts := httptest.NewServer(app.Router)
	defer ts.Close()

	client := ts.Client()
	adminToken := login(t, client, ts.URL, cfg.SeedAdminEmail, cfg.SeedAdminPassword)

	tenantID := getTenantID(t, app, cfg.SeedTenantName)
	employeeEmail := fmt.Sprintf("gdpr-employee-%d@example.com", time.Now().UnixNano())
	employeePassword := "Employee123!"
	employeeUserID := createUserWithRole(t, app, tenantID, auth.RoleEmployee, employeeEmail, employeePassword)
	employeeID := createEmployeeWithUser(t, client, ts.URL, adminToken, employeeUserID, "", employeeEmail)

	createRetentionPolicy(t, client, ts.URL, adminToken)
	runSummaries := runRetention(t, client, ts.URL, adminToken)
	if len(runSummaries) == 0 {
		t.Fatal("expected retention run summaries")
	}

	exportID, status := requestDSAR(t, client, ts.URL, adminToken, employeeID)
	if exportID == "" {
		t.Fatal("expected dsar export id")
	}
	if status == "" {
		t.Fatal("expected dsar status")
	}

	exports := listDSAR(t, client, ts.URL, adminToken)
	if len(exports) == 0 {
		t.Fatal("expected dsar exports in list")
	}
}

func testConfig(dbURL string) config.Config {
	return config.Config{
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
}

func getTenantID(t *testing.T, app *server.App, tenantName string) string {
	t.Helper()
	ctx := context.Background()
	var tenantID string
	if err := app.DB.QueryRow(ctx, "SELECT id FROM tenants WHERE name = $1", tenantName).Scan(&tenantID); err != nil {
		t.Fatalf("failed to load tenant: %v", err)
	}
	return tenantID
}

func createUserWithRole(t *testing.T, app *server.App, tenantID, roleName, email, password string) string {
	t.Helper()
	ctx := context.Background()
	var roleID string
	if err := app.DB.QueryRow(ctx, "SELECT id FROM roles WHERE tenant_id = $1 AND name = $2", tenantID, roleName).Scan(&roleID); err != nil {
		t.Fatalf("failed to load role: %v", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	var userID string
	if err := app.DB.QueryRow(ctx, `
    INSERT INTO users (tenant_id, email, password_hash, role_id)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, email, hash, roleID).Scan(&userID); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return userID
}

func createEmployeeWithUser(t *testing.T, client *http.Client, baseURL, token, userID, managerID, email string) string {
	t.Helper()
	payload := map[string]any{
		"userId":         userID,
		"firstName":      "Journey",
		"lastName":       "Tester",
		"email":          email,
		"status":         "active",
		"salary":         3500,
		"currency":       "USD",
		"employmentType": "full_time",
	}
	if managerID != "" {
		payload["managerId"] = managerID
	}
	resp := postJSON(t, client, baseURL+"/api/v1/employees", token, payload)
	var data map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode employee response: %v", err)
	}
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("expected employee id")
	}
	return id
}

func createReviewTemplate(t *testing.T, client *http.Client, baseURL, token string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/performance/review-templates", token, map[string]any{
		"name":        "Annual Review",
		"ratingScale": []map[string]any{{"value": 1, "label": "1"}, {"value": 2, "label": "2"}, {"value": 3, "label": "3"}},
		"questions":   []map[string]any{{"question": "How did you perform?"}},
	})
	var data map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode review template response: %v", err)
	}
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("expected review template id")
	}
	return id
}

func createReviewCycle(t *testing.T, client *http.Client, baseURL, token, templateID string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/performance/review-cycles", token, map[string]any{
		"name":       "Q1 Review",
		"startDate":  "2026-01-01",
		"endDate":    "2026-02-01",
		"templateId": templateID,
		"hrRequired": true,
	})
	var data map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode review cycle response: %v", err)
	}
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("expected review cycle id")
	}
	return id
}

func listReviewTasks(t *testing.T, client *http.Client, baseURL, token string) []map[string]any {
	t.Helper()
	resp := getJSON(t, client, baseURL+"/api/v1/performance/review-tasks", token)
	var data []map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode review tasks response: %v", err)
	}
	return data
}

func submitReviewResponse(t *testing.T, client *http.Client, baseURL, token, taskID, role string) {
	t.Helper()
	postJSON(t, client, fmt.Sprintf("%s/api/v1/performance/review-tasks/%s/responses", baseURL, taskID), token, map[string]any{
		"role":   role,
		"rating": 2,
		"responses": []map[string]any{
			{"question": "How did you perform?", "answer": "Solid"},
		},
	})
}

func finalizeReviewCycle(t *testing.T, client *http.Client, baseURL, token, cycleID string) string {
	t.Helper()
	resp := postJSON(t, client, fmt.Sprintf("%s/api/v1/performance/review-cycles/%s/finalize", baseURL, cycleID), token, map[string]any{})
	var data map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode finalize response: %v", err)
	}
	status, _ := data["status"].(string)
	return status
}

func findTaskForEmployee(tasks []map[string]any, employeeID string) string {
	for _, task := range tasks {
		if id, ok := task["id"].(string); ok {
			if employee, ok := task["employeeId"].(string); ok && employee == employeeID {
				return id
			}
		}
	}
	return ""
}

func createRetentionPolicy(t *testing.T, client *http.Client, baseURL, token string) {
	t.Helper()
	postJSON(t, client, baseURL+"/api/v1/gdpr/retention-policies", token, map[string]any{
		"dataCategory":  "leave",
		"retentionDays": 1,
	})
}

func runRetention(t *testing.T, client *http.Client, baseURL, token string) []map[string]any {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/gdpr/retention/run", token, map[string]any{
		"dataCategory": "leave",
	})
	var data []map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode retention run response: %v", err)
	}
	return data
}

func requestDSAR(t *testing.T, client *http.Client, baseURL, token, employeeID string) (string, string) {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/gdpr/dsar", token, map[string]any{
		"employeeId": employeeID,
	})
	var data map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode dsar response: %v", err)
	}
	id, _ := data["id"].(string)
	status, _ := data["status"].(string)
	return id, status
}

func listDSAR(t *testing.T, client *http.Client, baseURL, token string) []map[string]any {
	t.Helper()
	resp := getJSON(t, client, baseURL+"/api/v1/gdpr/dsar", token)
	var data []map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode dsar list response: %v", err)
	}
	return data
}
