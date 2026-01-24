package payroll

import (
	"context"
	"encoding/json"
	"time"
)

func (s *Service) ListSchedules(ctx context.Context, tenantID string) ([]Schedule, error) {
	return s.store.ListSchedules(ctx, tenantID)
}

func (s *Service) CreateSchedule(ctx context.Context, tenantID, name, frequency string, payDay int) (string, error) {
	return s.store.CreateSchedule(ctx, tenantID, name, frequency, payDay)
}

func (s *Service) ListGroups(ctx context.Context, tenantID string) ([]Group, error) {
	return s.store.ListGroups(ctx, tenantID)
}

func (s *Service) CreateGroup(ctx context.Context, tenantID, name, scheduleID, currency string) (string, error) {
	return s.store.CreateGroup(ctx, tenantID, name, scheduleID, currency)
}

func (s *Service) ListElements(ctx context.Context, tenantID string) ([]Element, error) {
	return s.store.ListElements(ctx, tenantID)
}

func (s *Service) CreateElement(ctx context.Context, tenantID string, element Element) (string, error) {
	return s.store.CreateElement(ctx, tenantID, element)
}

func (s *Service) ListJournalTemplates(ctx context.Context, tenantID string) ([]JournalTemplate, error) {
	return s.store.ListJournalTemplates(ctx, tenantID)
}

func (s *Service) CreateJournalTemplate(ctx context.Context, tenantID, name string, config map[string]any) (string, error) {
	if config == nil {
		config = map[string]any{}
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return s.store.CreateJournalTemplate(ctx, tenantID, name, configJSON)
}

func (s *Service) CountPeriods(ctx context.Context, tenantID string) (int, error) {
	return s.store.CountPeriods(ctx, tenantID)
}

func (s *Service) ListPeriods(ctx context.Context, tenantID string, limit, offset int) ([]Period, error) {
	return s.store.ListPeriods(ctx, tenantID, limit, offset)
}

func (s *Service) CreatePeriod(ctx context.Context, tenantID, scheduleID string, startDate, endDate time.Time) (string, error) {
	return s.store.CreatePeriod(ctx, tenantID, scheduleID, startDate, endDate)
}

func (s *Service) CountInputs(ctx context.Context, tenantID, periodID string) (int, error) {
	return s.store.CountInputs(ctx, tenantID, periodID)
}

func (s *Service) ListInputs(ctx context.Context, tenantID, periodID string, limit, offset int) ([]Input, error) {
	return s.store.ListInputs(ctx, tenantID, periodID, limit, offset)
}

func (s *Service) CreateInput(ctx context.Context, tenantID, periodID string, input Input) error {
	return s.store.CreateInput(ctx, tenantID, periodID, input)
}

func (s *Service) FindEmployeeIDByEmail(ctx context.Context, tenantID, email string) (string, error) {
	return s.store.FindEmployeeIDByEmail(ctx, tenantID, email)
}

func (s *Service) FindEmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	return s.store.FindEmployeeIDByUserID(ctx, tenantID, userID)
}

func (s *Service) GetPeriodDetails(ctx context.Context, tenantID, periodID string) (PeriodDetails, error) {
	return s.store.GetPeriodDetails(ctx, tenantID, periodID)
}

func (s *Service) CreateJobRun(ctx context.Context, tenantID, jobType string) (string, error) {
	return s.store.CreateJobRun(ctx, tenantID, jobType)
}

func (s *Service) UpdateJobRun(ctx context.Context, runID, status string, details any) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}
	return s.store.UpdateJobRun(ctx, runID, status, detailsJSON)
}

func (s *Service) ListActiveEmployeesForRun(ctx context.Context, tenantID, status string) ([]EmployeePayrollData, error) {
	return s.store.ListActiveEmployeesForRun(ctx, tenantID, status)
}

func (s *Service) ListInputLines(ctx context.Context, periodID, employeeID string) ([]InputLine, error) {
	return s.store.ListInputLines(ctx, periodID, employeeID)
}

func (s *Service) ListAdjustmentAmounts(ctx context.Context, tenantID, periodID, employeeID string, periodStart, periodEnd time.Time) ([]float64, error) {
	return s.store.ListAdjustmentAmounts(ctx, tenantID, periodID, employeeID, periodStart, periodEnd)
}

func (s *Service) ListUnpaidLeaves(ctx context.Context, tenantID, employeeID string, periodStart, periodEnd time.Time, status string) ([]LeaveWindow, error) {
	return s.store.ListUnpaidLeaves(ctx, tenantID, employeeID, periodStart, periodEnd, status)
}

func (s *Service) LatestNet(ctx context.Context, tenantID, employeeID string) (float64, error) {
	return s.store.LatestNet(ctx, tenantID, employeeID)
}

func (s *Service) UpsertPayrollResult(ctx context.Context, tenantID, periodID, employeeID string, gross, deductions, net float64, currency string, warningsJSON []byte) error {
	return s.store.UpsertPayrollResult(ctx, tenantID, periodID, employeeID, gross, deductions, net, currency, warningsJSON)
}

func (s *Service) UpdatePeriodStatus(ctx context.Context, tenantID, periodID, status string) error {
	return s.store.UpdatePeriodStatus(ctx, tenantID, periodID, status)
}

func (s *Service) FinalizePeriod(ctx context.Context, tenantID, periodID string) error {
	return s.store.FinalizePeriod(ctx, tenantID, periodID)
}

func (s *Service) CreatePayslipsForPeriod(ctx context.Context, periodID string) error {
	return s.store.CreatePayslipsForPeriod(ctx, periodID)
}

func (s *Service) ListPayslipIDs(ctx context.Context, tenantID, periodID string) ([]PayslipKey, error) {
	return s.store.ListPayslipIDs(ctx, tenantID, periodID)
}

func (s *Service) UpdatePayslipFileURL(ctx context.Context, payslipID, fileURL string) error {
	return s.store.UpdatePayslipFileURL(ctx, payslipID, fileURL)
}

func (s *Service) EmployeeUserID(ctx context.Context, tenantID, employeeID string) (string, error) {
	return s.store.EmployeeUserID(ctx, tenantID, employeeID)
}

func (s *Service) CountPayslips(ctx context.Context, tenantID, employeeID string) (int, error) {
	return s.store.CountPayslips(ctx, tenantID, employeeID)
}

func (s *Service) ListPayslips(ctx context.Context, tenantID, employeeID string, limit, offset int) ([]Payslip, error) {
	return s.store.ListPayslips(ctx, tenantID, employeeID, limit, offset)
}

func (s *Service) ListAdjustments(ctx context.Context, tenantID, periodID, employeeID string, limit, offset int) ([]Adjustment, int, error) {
	return s.store.ListAdjustments(ctx, tenantID, periodID, employeeID, limit, offset)
}

func (s *Service) CreateAdjustment(ctx context.Context, tenantID, periodID, employeeID, description string, amount float64, effectiveDate any) (string, error) {
	return s.store.CreateAdjustment(ctx, tenantID, periodID, employeeID, description, amount, effectiveDate)
}

func (s *Service) PeriodSummary(ctx context.Context, tenantID, periodID string) (PeriodSummary, error) {
	summary, warningsRaw, err := s.store.PeriodSummaryData(ctx, tenantID, periodID)
	if err != nil {
		return summary, err
	}
	summary.Warnings = map[string]int{}
	for _, raw := range warningsRaw {
		var warnings []string
		if err := json.Unmarshal(raw, &warnings); err != nil {
			continue
		}
		for _, key := range warnings {
			summary.Warnings[key]++
		}
	}
	return summary, nil
}

func (s *Service) PeriodStatus(ctx context.Context, tenantID, periodID string) (string, error) {
	return s.store.PeriodStatus(ctx, tenantID, periodID)
}

func (s *Service) DeleteResultsForPeriod(ctx context.Context, tenantID, periodID string) error {
	return s.store.DeleteResultsForPeriod(ctx, tenantID, periodID)
}

func (s *Service) DeletePayslipsForPeriod(ctx context.Context, tenantID, periodID string) error {
	return s.store.DeletePayslipsForPeriod(ctx, tenantID, periodID)
}

func (s *Service) RegisterRows(ctx context.Context, tenantID, periodID string) ([]RegisterRow, error) {
	return s.store.RegisterRows(ctx, tenantID, periodID)
}

func (s *Service) PeriodTotals(ctx context.Context, tenantID, periodID string) (float64, float64, float64, error) {
	return s.store.PeriodTotals(ctx, tenantID, periodID)
}

func (s *Service) JournalTemplateConfig(ctx context.Context, tenantID, templateID string) (map[string]any, error) {
	configJSON, err := s.store.JournalTemplateConfig(ctx, tenantID, templateID)
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return map[string]any{}, nil
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, nil
}

func (s *Service) PayslipInfo(ctx context.Context, tenantID, payslipID string) (string, string, error) {
	return s.store.PayslipInfo(ctx, tenantID, payslipID)
}

func (s *Service) PayslipPeriodID(ctx context.Context, tenantID, payslipID string) (string, error) {
	return s.store.PayslipPeriodID(ctx, tenantID, payslipID)
}

func (s *Service) PayslipEmployeePeriod(ctx context.Context, tenantID, payslipID string) (string, string, error) {
	return s.store.PayslipEmployeePeriod(ctx, tenantID, payslipID)
}
