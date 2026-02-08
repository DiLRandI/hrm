package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"hrm/internal/app/server"
)

func TestHighRiskEndpointsReturnValidationErrors(t *testing.T) {
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
	leaveTypeID := createLeaveType(t, client, ts.URL, adminToken)

	leavePolicyResp := postJSONStatus(t, client, ts.URL+"/api/v1/leave/policies", adminToken, map[string]any{
		"leaveTypeId":        leaveTypeID,
		"accrualRate":        1.5,
		"accrualPeriod":      "quarterly",
		"entitlement":        18,
		"carryOverLimit":     2,
		"allowNegative":      false,
		"requiresHrApproval": true,
	}, http.StatusBadRequest)
	assertValidationErrorField(t, leavePolicyResp, "accrualPeriod")

	scheduleID := createPayrollSchedule(t, client, ts.URL, adminToken)
	payrollPeriodResp := postJSONStatus(t, client, ts.URL+"/api/v1/payroll/periods", adminToken, map[string]any{
		"scheduleId": scheduleID,
		"startDate":  "2026-04-10",
		"endDate":    "2026-04-01",
	}, http.StatusBadRequest)
	assertValidationErrorField(t, payrollPeriodResp, "startDate")
	assertValidationErrorField(t, payrollPeriodResp, "endDate")

	retentionResp := postJSONStatus(t, client, ts.URL+"/api/v1/gdpr/retention-policies", adminToken, map[string]any{
		"dataCategory":  "unknown",
		"retentionDays": 0,
	}, http.StatusBadRequest)
	assertValidationErrorField(t, retentionResp, "dataCategory")
	assertValidationErrorField(t, retentionResp, "retentionDays")

	resetResp := postJSONStatus(t, client, ts.URL+"/api/v1/auth/reset", "", map[string]any{
		"token":       "",
		"newPassword": "weak",
	}, http.StatusBadRequest)
	assertValidationErrorField(t, resetResp, "token")
	assertValidationErrorField(t, resetResp, "newPassword")
}

func assertValidationErrorField(t *testing.T, env envelope, field string) {
	t.Helper()
	if code := envelopeErrorCode(env); code != "validation_error" {
		t.Fatalf("expected validation_error, got %+v", env.Error)
	}
	errMap, ok := env.Error.(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %T", env.Error)
	}
	details, ok := errMap["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected details object, got %+v", errMap["details"])
	}
	fieldsRaw, ok := details["fields"].([]any)
	if !ok {
		t.Fatalf("expected details.fields array, got %+v", details["fields"])
	}
	for _, item := range fieldsRaw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if value, _ := entry["field"].(string); value == field {
			return
		}
	}
	t.Fatalf("expected validation field %q in %+v", field, fieldsRaw)
}
