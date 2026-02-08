package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"hrm/internal/app/server"
	"hrm/internal/domain/auth"
)

func TestLeaveRequestRequiresDocumentAndSupportsHalfDay(t *testing.T) {
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

	employeeEmail := fmt.Sprintf("leave-doc-%d@example.com", time.Now().UnixNano())
	employeePassword := "Employee123!"
	employeeUserID := createUserWithRole(t, app, tenantID, auth.RoleEmployee, employeeEmail, employeePassword)
	employeeID := createEmployeeWithUser(t, client, ts.URL, adminToken, employeeUserID, "", employeeEmail)
	employeeToken := login(t, client, ts.URL, employeeEmail, employeePassword)

	leaveTypeID := createLeaveTypeWithDocRequirement(t, client, ts.URL, adminToken, true)

	failedCreate := postJSONStatus(t, client, ts.URL+"/api/v1/leave/requests", employeeToken, map[string]any{
		"leaveTypeId": leaveTypeID,
		"startDate":   "2026-03-10",
		"endDate":     "2026-03-10",
		"startHalf":   true,
		"endHalf":     false,
		"reason":      "Medical appointment",
	}, http.StatusBadRequest)
	if code := envelopeErrorCode(failedCreate); code != "document_required" {
		t.Fatalf("expected document_required error, got %+v", failedCreate.Error)
	}

	createResp := postMultipartStatus(t, client, ts.URL+"/api/v1/leave/requests", employeeToken, map[string]string{
		"leaveTypeId": leaveTypeID,
		"startDate":   "2026-03-10",
		"endDate":     "2026-03-10",
		"startHalf":   "true",
		"endHalf":     "false",
		"reason":      "Medical appointment",
	}, "documents", "medical-proof.txt", []byte("clinic slip"), http.StatusCreated)
	var created map[string]any
	if err := json.Unmarshal(createResp.Data, &created); err != nil {
		t.Fatalf("failed to decode create leave response: %v", err)
	}
	requestID, _ := created["id"].(string)
	if requestID == "" {
		t.Fatal("expected leave request id")
	}

	requestListResp := getJSON(t, client, ts.URL+"/api/v1/leave/requests?limit=25&offset=0", employeeToken)
	var requests []map[string]any
	if err := json.Unmarshal(requestListResp.Data, &requests); err != nil {
		t.Fatalf("failed to decode leave request list: %v", err)
	}
	var createdReq map[string]any
	for _, req := range requests {
		if id, _ := req["id"].(string); id == requestID {
			createdReq = req
			break
		}
	}
	if createdReq == nil {
		t.Fatalf("expected request %s in list", requestID)
	}
	startHalf, _ := createdReq["startHalf"].(bool)
	endHalf, _ := createdReq["endHalf"].(bool)
	if !startHalf || endHalf {
		t.Fatalf("expected startHalf=true and endHalf=false, got startHalf=%v endHalf=%v", startHalf, endHalf)
	}
	days, _ := createdReq["days"].(float64)
	if days != 0.5 {
		t.Fatalf("expected 0.5 days for single half-day request, got %v", days)
	}
	docs, _ := createdReq["documents"].([]any)
	if len(docs) == 0 {
		t.Fatal("expected request documents in list response")
	}

	postJSON(t, client, ts.URL+"/api/v1/leave/requests/"+requestID+"/approve", adminToken, map[string]any{})

	secondCreateResp := postMultipartStatus(t, client, ts.URL+"/api/v1/leave/requests", employeeToken, map[string]string{
		"leaveTypeId": leaveTypeID,
		"startDate":   "2026-03-12",
		"endDate":     "2026-03-12",
		"startHalf":   "false",
		"endHalf":     "true",
		"reason":      "Follow-up",
	}, "documents", "medical-proof-2.txt", []byte("clinic slip 2"), http.StatusCreated)
	var secondCreated map[string]any
	if err := json.Unmarshal(secondCreateResp.Data, &secondCreated); err != nil {
		t.Fatalf("failed to decode second create leave response: %v", err)
	}
	secondRequestID, _ := secondCreated["id"].(string)
	if secondRequestID == "" {
		t.Fatal("expected second leave request id")
	}
	postJSON(t, client, ts.URL+"/api/v1/leave/requests/"+secondRequestID+"/reject", adminToken, map[string]any{})

	balancesResp := getJSON(t, client, ts.URL+"/api/v1/leave/balances?employeeId="+employeeID, adminToken)
	var balances []map[string]any
	if err := json.Unmarshal(balancesResp.Data, &balances); err != nil {
		t.Fatalf("failed to decode leave balances: %v", err)
	}
	var balanceRow map[string]any
	for _, row := range balances {
		if leaveType, _ := row["leaveTypeId"].(string); leaveType == leaveTypeID {
			balanceRow = row
			break
		}
	}
	if balanceRow == nil {
		t.Fatalf("expected balance row for leaveType %s", leaveTypeID)
	}
	used, _ := balanceRow["used"].(float64)
	pending, _ := balanceRow["pending"].(float64)
	if used != 0.5 || pending != 0 {
		t.Fatalf("expected used=0.5 pending=0 after approval+rejection flow, got used=%v pending=%v", used, pending)
	}
}

func TestLeaveRequestRejectsInvalidSingleDayHalfDayCombination(t *testing.T) {
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

	employeeEmail := fmt.Sprintf("leave-half-%d@example.com", time.Now().UnixNano())
	employeePassword := "Employee123!"
	employeeUserID := createUserWithRole(t, app, tenantID, auth.RoleEmployee, employeeEmail, employeePassword)
	_ = createEmployeeWithUser(t, client, ts.URL, adminToken, employeeUserID, "", employeeEmail)
	employeeToken := login(t, client, ts.URL, employeeEmail, employeePassword)

	leaveTypeID := createLeaveTypeWithDocRequirement(t, client, ts.URL, adminToken, false)

	resp := postJSONStatus(t, client, ts.URL+"/api/v1/leave/requests", employeeToken, map[string]any{
		"leaveTypeId": leaveTypeID,
		"startDate":   "2026-03-10",
		"endDate":     "2026-03-10",
		"startHalf":   true,
		"endHalf":     true,
		"reason":      "Invalid combo",
	}, http.StatusBadRequest)
	if code := envelopeErrorCode(resp); code != "invalid_dates" {
		t.Fatalf("expected invalid_dates for single-day startHalf+endHalf, got %+v", resp.Error)
	}
}

func createLeaveTypeWithDocRequirement(t *testing.T, client *http.Client, baseURL, token string, requiresDoc bool) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/leave/types", token, map[string]any{
		"name":        fmt.Sprintf("Medical-%d", time.Now().UnixNano()),
		"code":        fmt.Sprintf("MED%d", time.Now().UnixNano()%100000),
		"isPaid":      true,
		"requiresDoc": requiresDoc,
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

func postJSONStatus(t *testing.T, client *http.Client, url, token string, body any, want int) envelope {
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
	if resp.StatusCode != want {
		t.Fatalf("expected status %d, got %d: %s", want, resp.StatusCode, string(raw))
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return env
}

func postMultipartStatus(t *testing.T, client *http.Client, url, token string, fields map[string]string, fileField, fileName string, content []byte, want int) envelope {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("failed to write field %s: %v", key, err)
		}
	}
	fileWriter, err := writer.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err := fileWriter.Write(content); err != nil {
		t.Fatalf("failed to write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
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

func envelopeErrorCode(env envelope) string {
	if env.Error == nil {
		return ""
	}
	if m, ok := env.Error.(map[string]any); ok {
		if code, ok := m["code"].(string); ok {
			return code
		}
	}
	return ""
}
