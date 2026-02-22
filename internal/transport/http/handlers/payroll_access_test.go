package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"hrm/internal/app/server"
	"hrm/internal/domain/auth"
)

func TestEmployeePayrollAccessIsLimitedToPayslips(t *testing.T) {
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
	hrToken := login(t, client, ts.URL, cfg.SeedAdminEmail, cfg.SeedAdminPassword)

	employeeEmail := fmt.Sprintf("payroll-access-%d@example.com", time.Now().UnixNano())
	employeeAccount := createUserAccountWithEmployee(t, client, ts.URL, hrToken, auth.RoleEmployee, employeeEmail, "")
	employeeToken := login(t, client, ts.URL, employeeEmail, employeeAccount.TempPassword)

	scheduleID := createPayrollSchedule(t, client, ts.URL, hrToken)
	periodID := createPayrollPeriod(t, client, ts.URL, hrToken, scheduleID)

	getJSONStatus(t, client, ts.URL+"/api/v1/payroll/payslips", employeeToken, http.StatusOK)
	getJSONStatus(t, client, ts.URL+"/api/v1/payroll/periods", employeeToken, http.StatusForbidden)
	getJSONStatus(t, client, ts.URL+"/api/v1/payroll/periods/"+periodID+"/inputs", employeeToken, http.StatusForbidden)
	getJSONStatus(t, client, ts.URL+"/api/v1/payroll/periods/"+periodID+"/adjustments", employeeToken, http.StatusForbidden)
	getJSONStatus(t, client, ts.URL+"/api/v1/payroll/periods/"+periodID+"/export/register", employeeToken, http.StatusForbidden)
	getJSONStatus(t, client, ts.URL+"/api/v1/payroll/periods/"+periodID+"/export/journal", employeeToken, http.StatusForbidden)
}
