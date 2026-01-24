package payroll

import (
	"context"
	"time"
)

type StoreAPI interface {
	ListSchedules(ctx context.Context, tenantID string) ([]Schedule, error)
	CreateSchedule(ctx context.Context, tenantID, name, frequency string, payDay int) (string, error)
	ListGroups(ctx context.Context, tenantID string) ([]Group, error)
	CreateGroup(ctx context.Context, tenantID, name, scheduleID, currency string) (string, error)
	ListElements(ctx context.Context, tenantID string) ([]Element, error)
	CreateElement(ctx context.Context, tenantID string, element Element) (string, error)
	ListJournalTemplates(ctx context.Context, tenantID string) ([]JournalTemplate, error)
	CreateJournalTemplate(ctx context.Context, tenantID, name string, configJSON []byte) (string, error)
	CountPeriods(ctx context.Context, tenantID string) (int, error)
	ListPeriods(ctx context.Context, tenantID string, limit, offset int) ([]Period, error)
	CreatePeriod(ctx context.Context, tenantID, scheduleID string, startDate, endDate time.Time) (string, error)
	CountInputs(ctx context.Context, tenantID, periodID string) (int, error)
	ListInputs(ctx context.Context, tenantID, periodID string, limit, offset int) ([]Input, error)
	CreateInput(ctx context.Context, tenantID, periodID string, input Input) error
	FindEmployeeIDByEmail(ctx context.Context, tenantID, email string) (string, error)
	FindEmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error)
	GetPeriodDetails(ctx context.Context, tenantID, periodID string) (PeriodDetails, error)
	CreateJobRun(ctx context.Context, tenantID, jobType string) (string, error)
	UpdateJobRun(ctx context.Context, runID, status string, detailsJSON []byte) error
	ListActiveEmployeesForRun(ctx context.Context, tenantID, status string) ([]EmployeePayrollData, error)
	ListInputLines(ctx context.Context, periodID, employeeID string) ([]InputLine, error)
	ListAdjustmentAmounts(ctx context.Context, tenantID, periodID, employeeID string, periodStart, periodEnd time.Time) ([]float64, error)
	ListUnpaidLeaves(ctx context.Context, tenantID, employeeID string, periodStart, periodEnd time.Time, status string) ([]LeaveWindow, error)
	LatestNet(ctx context.Context, tenantID, employeeID string) (float64, error)
	UpsertPayrollResult(ctx context.Context, tenantID, periodID, employeeID string, gross, deductions, net float64, currency string, warningsJSON []byte) error
	UpdatePeriodStatus(ctx context.Context, tenantID, periodID, status string) error
	FinalizePeriod(ctx context.Context, tenantID, periodID string) error
	CreatePayslipsForPeriod(ctx context.Context, periodID string) error
	ListPayslipIDs(ctx context.Context, tenantID, periodID string) ([]PayslipKey, error)
	UpdatePayslipFileURL(ctx context.Context, payslipID, fileURL string) error
	EmployeeUserID(ctx context.Context, tenantID, employeeID string) (string, error)
	CountPayslips(ctx context.Context, tenantID, employeeID string) (int, error)
	ListPayslips(ctx context.Context, tenantID, employeeID string, limit, offset int) ([]Payslip, error)
	ListAdjustments(ctx context.Context, tenantID, periodID, employeeID string, limit, offset int) ([]Adjustment, int, error)
	CreateAdjustment(ctx context.Context, tenantID, periodID, employeeID, description string, amount float64, effectiveDate any) (string, error)
	PeriodSummaryData(ctx context.Context, tenantID, periodID string) (PeriodSummary, [][]byte, error)
	PeriodStatus(ctx context.Context, tenantID, periodID string) (string, error)
	DeleteResultsForPeriod(ctx context.Context, tenantID, periodID string) error
	DeletePayslipsForPeriod(ctx context.Context, tenantID, periodID string) error
	RegisterRows(ctx context.Context, tenantID, periodID string) ([]RegisterRow, error)
	PeriodTotals(ctx context.Context, tenantID, periodID string) (float64, float64, float64, error)
	JournalTemplateConfig(ctx context.Context, tenantID, templateID string) ([]byte, error)
	PayslipInfo(ctx context.Context, tenantID, payslipID string) (string, string, error)
	PayslipPeriodID(ctx context.Context, tenantID, payslipID string) (string, error)
	PayslipEmployeePeriod(ctx context.Context, tenantID, payslipID string) (string, string, error)
	PayslipPDFData(ctx context.Context, tenantID, periodID, employeeID string) (PayslipPDFData, error)
}
