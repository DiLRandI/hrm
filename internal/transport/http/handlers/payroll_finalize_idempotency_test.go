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
	"sync"
	"testing"
	"time"

	"hrm/internal/app/server"
	"hrm/internal/domain/payroll"
)

func TestPayrollFinalizeRequiresIdempotencyAndReplays(t *testing.T) {
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
	employeeEmail := fmt.Sprintf("finalize-replay-%d@example.com", time.Now().UnixNano())
	_ = createEmployee(t, client, ts.URL, adminToken, employeeEmail)

	scheduleID := createPayrollSchedule(t, client, ts.URL, adminToken)
	periodID := createPayrollPeriodWithRange(t, client, ts.URL, adminToken, scheduleID, "2026-01-01", "2026-01-31")
	runStatus := runPayroll(t, client, ts.URL, adminToken, periodID)
	if runStatus != payroll.PeriodStatusReviewed {
		t.Fatalf("expected reviewed payroll period before finalize, got %s", runStatus)
	}

	missingKeyResp := postJSONStatus(t, client, ts.URL+"/api/v1/payroll/periods/"+periodID+"/finalize", adminToken, map[string]any{}, http.StatusBadRequest)
	if code := envelopeErrorCode(missingKeyResp); code != "validation_error" {
		t.Fatalf("expected validation_error for missing idempotency key, got %+v", missingKeyResp.Error)
	}

	firstStatus, firstEnv := postJSONAnyStatusWithHeaders(t, client, ts.URL+"/api/v1/payroll/periods/"+periodID+"/finalize", adminToken, map[string]any{}, map[string]string{
		"Idempotency-Key": "finalize-replay-key",
	})
	if firstStatus != http.StatusOK {
		t.Fatalf("expected 200 for first finalize, got %d", firstStatus)
	}
	if status := envelopeDataStatus(t, firstEnv); status != payroll.PeriodStatusFinalized {
		t.Fatalf("expected finalized status on first finalize, got %s", status)
	}

	var firstPayslipCount int
	if err := app.DB.QueryRow(context.Background(), "SELECT COUNT(1) FROM payslips WHERE period_id = $1", periodID).Scan(&firstPayslipCount); err != nil {
		t.Fatalf("failed to count payslips after first finalize: %v", err)
	}
	if firstPayslipCount == 0 {
		t.Fatal("expected payslips after first finalize")
	}

	replayStatus, replayEnv := postJSONAnyStatusWithHeaders(t, client, ts.URL+"/api/v1/payroll/periods/"+periodID+"/finalize", adminToken, map[string]any{}, map[string]string{
		"Idempotency-Key": "finalize-replay-key",
	})
	if replayStatus != http.StatusOK {
		t.Fatalf("expected 200 for idempotent replay, got %d", replayStatus)
	}
	if status := envelopeDataStatus(t, replayEnv); status != payroll.PeriodStatusFinalized {
		t.Fatalf("expected finalized status on replay, got %s", status)
	}

	var replayPayslipCount int
	if err := app.DB.QueryRow(context.Background(), "SELECT COUNT(1) FROM payslips WHERE period_id = $1", periodID).Scan(&replayPayslipCount); err != nil {
		t.Fatalf("failed to count payslips after replay finalize: %v", err)
	}
	if replayPayslipCount != firstPayslipCount {
		t.Fatalf("expected idempotent replay to preserve payslip count %d, got %d", firstPayslipCount, replayPayslipCount)
	}
}

func TestPayrollFinalizeIdempotencyConflictAcrossPeriods(t *testing.T) {
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
	employeeEmail := fmt.Sprintf("finalize-conflict-%d@example.com", time.Now().UnixNano())
	_ = createEmployee(t, client, ts.URL, adminToken, employeeEmail)

	scheduleID := createPayrollSchedule(t, client, ts.URL, adminToken)
	periodOne := createPayrollPeriodWithRange(t, client, ts.URL, adminToken, scheduleID, "2026-02-01", "2026-02-28")
	if runPayroll(t, client, ts.URL, adminToken, periodOne) != payroll.PeriodStatusReviewed {
		t.Fatalf("expected period %s to be reviewed before finalize", periodOne)
	}

	firstFinalizeStatus, _ := postJSONAnyStatusWithHeaders(t, client, ts.URL+"/api/v1/payroll/periods/"+periodOne+"/finalize", adminToken, map[string]any{}, map[string]string{
		"Idempotency-Key": "shared-key",
	})
	if firstFinalizeStatus != http.StatusOK {
		t.Fatalf("expected first finalize to succeed, got %d", firstFinalizeStatus)
	}

	periodTwo := createPayrollPeriodWithRange(t, client, ts.URL, adminToken, scheduleID, "2026-03-01", "2026-03-31")
	if runPayroll(t, client, ts.URL, adminToken, periodTwo) != payroll.PeriodStatusReviewed {
		t.Fatalf("expected period %s to be reviewed before finalize", periodTwo)
	}

	conflictStatus, conflictEnv := postJSONAnyStatusWithHeaders(t, client, ts.URL+"/api/v1/payroll/periods/"+periodTwo+"/finalize", adminToken, map[string]any{}, map[string]string{
		"Idempotency-Key": "shared-key",
	})
	if conflictStatus != http.StatusConflict {
		t.Fatalf("expected 409 for shared idempotency key conflict, got %d", conflictStatus)
	}
	if code := envelopeErrorCode(conflictEnv); code != "idempotency_conflict" {
		t.Fatalf("expected idempotency_conflict code, got %+v", conflictEnv.Error)
	}
}

func TestPayrollFinalizeRollsBackWhenNoResultsExist(t *testing.T) {
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
	scheduleID := createPayrollSchedule(t, client, ts.URL, adminToken)
	periodID := createPayrollPeriodWithRange(t, client, ts.URL, adminToken, scheduleID, "2026-04-01", "2026-04-30")

	if _, err := app.DB.Exec(context.Background(), "UPDATE payroll_periods SET status = $1 WHERE id = $2", payroll.PeriodStatusReviewed, periodID); err != nil {
		t.Fatalf("failed to force period to reviewed state: %v", err)
	}

	finalizeStatus, finalizeEnv := postJSONAnyStatusWithHeaders(t, client, ts.URL+"/api/v1/payroll/periods/"+periodID+"/finalize", adminToken, map[string]any{}, map[string]string{
		"Idempotency-Key": "rollback-check-key",
	})
	if finalizeStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 when finalizing reviewed period with no results, got %d", finalizeStatus)
	}
	if code := envelopeErrorCode(finalizeEnv); code != "invalid_state" {
		t.Fatalf("expected invalid_state for finalize with no results, got %+v", finalizeEnv.Error)
	}

	var currentStatus string
	if err := app.DB.QueryRow(context.Background(), "SELECT status FROM payroll_periods WHERE id = $1", periodID).Scan(&currentStatus); err != nil {
		t.Fatalf("failed to read payroll period status: %v", err)
	}
	if currentStatus != payroll.PeriodStatusReviewed {
		t.Fatalf("expected rollback to keep reviewed status, got %s", currentStatus)
	}
}

func TestPayrollFinalizeConcurrentRequestsResolveDeterministically(t *testing.T) {
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
	employeeEmail := fmt.Sprintf("finalize-concurrent-%d@example.com", time.Now().UnixNano())
	_ = createEmployee(t, client, ts.URL, adminToken, employeeEmail)

	scheduleID := createPayrollSchedule(t, client, ts.URL, adminToken)
	periodID := createPayrollPeriodWithRange(t, client, ts.URL, adminToken, scheduleID, "2026-05-01", "2026-05-31")
	if runPayroll(t, client, ts.URL, adminToken, periodID) != payroll.PeriodStatusReviewed {
		t.Fatalf("expected reviewed payroll period before finalize")
	}

	type result struct {
		status int
		env    envelope
		err    error
	}

	results := make(chan result, 2)
	keys := []string{"concurrent-key-1", "concurrent-key-2"}
	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		go func(idempotencyKey string) {
			defer wg.Done()
			status, env, err := postJSONAnyStatusWithHeadersNoFail(client, ts.URL+"/api/v1/payroll/periods/"+periodID+"/finalize", adminToken, map[string]any{}, map[string]string{
				"Idempotency-Key": idempotencyKey,
			})
			results <- result{status: status, env: env, err: err}
		}(key)
	}
	wg.Wait()
	close(results)

	successCount := 0
	invalidStateCount := 0
	for item := range results {
		if item.err != nil {
			t.Fatalf("concurrent finalize request failed: %v", item.err)
		}
		if item.status == http.StatusOK {
			successCount++
			continue
		}
		if item.status == http.StatusBadRequest && envelopeErrorCode(item.env) == "invalid_state" {
			invalidStateCount++
			continue
		}
		t.Fatalf("unexpected concurrent finalize response: status=%d err=%+v", item.status, item.env.Error)
	}

	if successCount != 1 || invalidStateCount != 1 {
		t.Fatalf("expected one success and one invalid_state; got success=%d invalid_state=%d", successCount, invalidStateCount)
	}
}

func createPayrollPeriodWithRange(t *testing.T, client *http.Client, baseURL, token, scheduleID, startDate, endDate string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/payroll/periods", token, map[string]any{
		"scheduleId": scheduleID,
		"startDate":  startDate,
		"endDate":    endDate,
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

func postJSONAnyStatusWithHeaders(t *testing.T, client *http.Client, url, token string, body any, headers map[string]string) (int, envelope) {
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
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("failed to decode envelope %q: %v", string(raw), err)
	}
	return resp.StatusCode, env
}

func envelopeDataStatus(t *testing.T, env envelope) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("failed to decode envelope data: %v", err)
	}
	status, _ := payload["status"].(string)
	return status
}

func postJSONAnyStatusWithHeadersNoFail(client *http.Client, url, token string, body any, headers map[string]string) (int, envelope, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, envelope{}, err
		}
		reader = bytes.NewBuffer(raw)
	}

	req, err := http.NewRequest(http.MethodPost, url, reader)
	if err != nil {
		return 0, envelope{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, envelope{}, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, envelope{}, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, envelope{}, err
	}
	return resp.StatusCode, env, nil
}
