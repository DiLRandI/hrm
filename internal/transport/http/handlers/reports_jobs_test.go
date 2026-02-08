package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"hrm/internal/app/server"
)

func TestReportsJobRunsFilteringPaginationAndDetails(t *testing.T) {
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

	jobOneID := insertJobRun(t, app, tenantID, "gdpr_retention", "failed", map[string]any{"error": "retention query failed"}, time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC))
	_ = insertJobRun(t, app, tenantID, "payroll_run", "completed", map[string]any{"processed": 28}, time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC))
	_ = insertJobRun(t, app, tenantID, "leave_accrual", "running", map[string]any{"processed": 3}, time.Date(2026, time.January, 3, 10, 0, 0, 0, time.UTC))

	listEnv, total := getJSONWithMetaStatus(t, client, ts.URL+"/api/v1/reports/jobs?limit=2&offset=0", adminToken, http.StatusOK)
	if total != 3 {
		t.Fatalf("expected total 3 job runs, got %d", total)
	}
	allRuns := envelopeDataSlice(t, listEnv)
	if len(allRuns) != 2 {
		t.Fatalf("expected 2 runs in paginated list, got %d", len(allRuns))
	}

	filterEnv, filteredTotal := getJSONWithMetaStatus(t, client, ts.URL+"/api/v1/reports/jobs?jobType=gdpr_retention&status=failed", adminToken, http.StatusOK)
	if filteredTotal != 1 {
		t.Fatalf("expected 1 filtered run, got %d", filteredTotal)
	}
	filteredRuns := envelopeDataSlice(t, filterEnv)
	if len(filteredRuns) != 1 {
		t.Fatalf("expected one filtered row, got %d", len(filteredRuns))
	}
	if id, _ := filteredRuns[0]["id"].(string); id != jobOneID {
		t.Fatalf("expected filtered run id %s, got %v", jobOneID, filteredRuns[0]["id"])
	}
	details, _ := filteredRuns[0]["details"].(map[string]any)
	if msg, _ := details["error"].(string); msg == "" {
		t.Fatalf("expected filtered run details to include error message, got %+v", details)
	}

	dateEnv, dateTotal := getJSONWithMetaStatus(t, client, ts.URL+"/api/v1/reports/jobs?startedFrom=2026-01-02&startedTo=2026-01-02", adminToken, http.StatusOK)
	if dateTotal != 1 {
		t.Fatalf("expected 1 run in date filter window, got %d", dateTotal)
	}
	dateRuns := envelopeDataSlice(t, dateEnv)
	if len(dateRuns) != 1 {
		t.Fatalf("expected one row in date window, got %d", len(dateRuns))
	}

	detailEnv := getJSONStatus(t, client, ts.URL+"/api/v1/reports/jobs/"+jobOneID, adminToken, http.StatusOK)
	detailData := envelopeDataMap(t, detailEnv)
	if id, _ := detailData["id"].(string); id != jobOneID {
		t.Fatalf("expected job detail id %s, got %v", jobOneID, detailData["id"])
	}
	detailMap, _ := detailData["details"].(map[string]any)
	if msg, _ := detailMap["error"].(string); msg == "" {
		t.Fatalf("expected detail endpoint to include error details, got %+v", detailMap)
	}
}

func insertJobRun(t *testing.T, app *server.App, tenantID, jobType, status string, details map[string]any, startedAt time.Time) string {
	t.Helper()
	detailsRaw, err := json.Marshal(details)
	if err != nil {
		t.Fatalf("failed to marshal job details: %v", err)
	}
	completedAt := startedAt.Add(10 * time.Minute)
	if status == "running" {
		completedAt = time.Time{}
	}

	var runID string
	if status == "running" {
		if err := app.DB.QueryRow(context.Background(), `
      INSERT INTO job_runs (tenant_id, job_type, status, details_json, started_at, completed_at)
      VALUES ($1, $2, $3, $4, $5, NULL)
      RETURNING id
    `, tenantID, jobType, status, detailsRaw, startedAt).Scan(&runID); err != nil {
			t.Fatalf("failed to insert running job run: %v", err)
		}
		return runID
	}

	if err := app.DB.QueryRow(context.Background(), `
    INSERT INTO job_runs (tenant_id, job_type, status, details_json, started_at, completed_at)
    VALUES ($1, $2, $3, $4, $5, $6)
    RETURNING id
  `, tenantID, jobType, status, detailsRaw, startedAt, completedAt).Scan(&runID); err != nil {
		t.Fatalf("failed to insert job run: %v", err)
	}
	return runID
}

func getJSONWithMetaStatus(t *testing.T, client *http.Client, url, token string, want int) (envelope, int) {
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

	totalHeader := resp.Header.Get("X-Total-Count")
	total, err := strconv.Atoi(totalHeader)
	if err != nil {
		t.Fatalf("expected X-Total-Count header, got %q", totalHeader)
	}
	return env, total
}

func envelopeDataSlice(t *testing.T, env envelope) []map[string]any {
	t.Helper()
	var payload []map[string]any
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("failed to decode array payload: %v", err)
	}
	return payload
}

func envelopeDataMap(t *testing.T, env envelope) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("failed to decode object payload: %v", err)
	}
	return payload
}
