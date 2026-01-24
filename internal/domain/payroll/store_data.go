package payroll

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

func (s *Store) ListSchedules(ctx context.Context, tenantID string) ([]Schedule, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, name, frequency, pay_day, created_at
    FROM pay_schedules
    WHERE tenant_id = $1
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []Schedule
	for rows.Next() {
		var schedule Schedule
		if err := rows.Scan(&schedule.ID, &schedule.Name, &schedule.Frequency, &schedule.PayDay, &schedule.CreatedAt); err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	return schedules, nil
}

func (s *Store) CreateSchedule(ctx context.Context, tenantID, name, frequency string, payDay int) (string, error) {
	var id string
	err := s.DB.QueryRow(ctx, `
    INSERT INTO pay_schedules (tenant_id, name, frequency, pay_day)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, name, frequency, payDay).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) ListGroups(ctx context.Context, tenantID string) ([]Group, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, name, COALESCE(schedule_id::text, ''), COALESCE(currency, 'USD')
    FROM pay_groups
    WHERE tenant_id = $1
    ORDER BY name
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.Name, &group.ScheduleID, &group.Currency); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func (s *Store) CreateGroup(ctx context.Context, tenantID, name, scheduleID, currency string) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO pay_groups (tenant_id, name, schedule_id, currency)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, name, nullIfEmpty(scheduleID), currency).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) ListElements(ctx context.Context, tenantID string) ([]Element, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, name, element_type, calc_type, amount, taxable
    FROM pay_elements
    WHERE tenant_id = $1
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var elements []Element
	for rows.Next() {
		var element Element
		if err := rows.Scan(&element.ID, &element.Name, &element.ElementType, &element.CalcType, &element.Amount, &element.Taxable); err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
	return elements, nil
}

func (s *Store) CreateElement(ctx context.Context, tenantID string, element Element) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO pay_elements (tenant_id, name, element_type, calc_type, amount, taxable)
    VALUES ($1,$2,$3,$4,$5,$6)
    RETURNING id
  `, tenantID, element.Name, element.ElementType, element.CalcType, element.Amount, element.Taxable).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) ListJournalTemplates(ctx context.Context, tenantID string) ([]JournalTemplate, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, name, config_json
    FROM journal_templates
    WHERE tenant_id = $1
    ORDER BY name
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []JournalTemplate
	for rows.Next() {
		var template JournalTemplate
		var configJSON []byte
		if err := rows.Scan(&template.ID, &template.Name, &configJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(configJSON, &template.Config); err != nil {
			template.Config = map[string]any{}
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func (s *Store) CreateJournalTemplate(ctx context.Context, tenantID, name string, configJSON []byte) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO journal_templates (tenant_id, name, config_json)
    VALUES ($1,$2,$3)
    RETURNING id
  `, tenantID, name, configJSON).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) CountPeriods(ctx context.Context, tenantID string) (int, error) {
	var total int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM payroll_periods WHERE tenant_id = $1", tenantID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) ListPeriods(ctx context.Context, tenantID string, limit, offset int) ([]Period, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, schedule_id, start_date, end_date, status
    FROM payroll_periods
    WHERE tenant_id = $1
    ORDER BY start_date DESC
    LIMIT $2 OFFSET $3
  `, tenantID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var periods []Period
	for rows.Next() {
		var period Period
		if err := rows.Scan(&period.ID, &period.ScheduleID, &period.StartDate, &period.EndDate, &period.Status); err != nil {
			return nil, err
		}
		periods = append(periods, period)
	}
	return periods, nil
}

func (s *Store) CreatePeriod(ctx context.Context, tenantID, scheduleID string, startDate, endDate time.Time) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO payroll_periods (tenant_id, schedule_id, start_date, end_date)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, scheduleID, startDate, endDate).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) CountInputs(ctx context.Context, tenantID, periodID string) (int, error) {
	var total int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM payroll_inputs WHERE tenant_id = $1 AND period_id = $2", tenantID, periodID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) ListInputs(ctx context.Context, tenantID, periodID string, limit, offset int) ([]Input, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT employee_id, element_id, units, rate, amount, source
    FROM payroll_inputs
    WHERE tenant_id = $1 AND period_id = $2
    ORDER BY created_at DESC
    LIMIT $3 OFFSET $4
  `, tenantID, periodID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var inputs []Input
	for rows.Next() {
		var input Input
		if err := rows.Scan(&input.EmployeeID, &input.ElementID, &input.Units, &input.Rate, &input.Amount, &input.Source); err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

func (s *Store) CreateInput(ctx context.Context, tenantID, periodID string, input Input) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO payroll_inputs (tenant_id, period_id, employee_id, element_id, units, rate, amount, source)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
  `, tenantID, periodID, input.EmployeeID, input.ElementID, input.Units, input.Rate, input.Amount, input.Source)
	return err
}

func (s *Store) FindEmployeeIDByEmail(ctx context.Context, tenantID, email string) (string, error) {
	var employeeID string
	if err := s.DB.QueryRow(ctx, "SELECT id FROM employees WHERE tenant_id = $1 AND email = $2", tenantID, email).Scan(&employeeID); err != nil {
		return "", err
	}
	return employeeID, nil
}

func (s *Store) FindEmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	var employeeID string
	if err := s.DB.QueryRow(ctx, "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", tenantID, userID).Scan(&employeeID); err != nil {
		return "", err
	}
	return employeeID, nil
}

func (s *Store) GetPeriodDetails(ctx context.Context, tenantID, periodID string) (PeriodDetails, error) {
	var details PeriodDetails
	err := s.DB.QueryRow(ctx, `
    SELECT status, start_date, end_date, schedule_id
    FROM payroll_periods
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, periodID).Scan(&details.Status, &details.StartDate, &details.EndDate, &details.ScheduleID)
	return details, err
}

func (s *Store) CreateJobRun(ctx context.Context, tenantID, jobType string) (string, error) {
	var runID string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO job_runs (tenant_id, job_type, status)
    VALUES ($1,$2,$3)
    RETURNING id
  `, tenantID, jobType, "running").Scan(&runID); err != nil {
		return "", err
	}
	return runID, nil
}

func (s *Store) UpdateJobRun(ctx context.Context, runID, status string, detailsJSON []byte) error {
	if detailsJSON == nil {
		detailsJSON = []byte("{}")
	}
	_, execErr := s.DB.Exec(ctx, `
    UPDATE job_runs SET status = $1, details_json = $2, completed_at = now()
    WHERE id = $3
  `, status, detailsJSON, runID)
	return execErr
}

func (s *Store) ListActiveEmployeesForRun(ctx context.Context, tenantID, status string) ([]EmployeePayrollData, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT e.id,
           e.salary,
           e.salary_enc,
           COALESCE(pg.currency, e.currency),
           COALESCE(e.bank_account, ''),
           e.bank_account_enc,
           COALESCE(pg.schedule_id::text, '')
    FROM employees e
    LEFT JOIN pay_groups pg ON e.pay_group_id = pg.id
    WHERE e.tenant_id = $1 AND e.status = $2
  `, tenantID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EmployeePayrollData
	for rows.Next() {
		var employee EmployeePayrollData
		if err := rows.Scan(&employee.EmployeeID, &employee.SalaryPlain, &employee.SalaryEnc, &employee.Currency, &employee.BankPlain, &employee.BankEnc, &employee.GroupScheduleID); err != nil {
			return nil, err
		}
		out = append(out, employee)
	}
	return out, nil
}

func (s *Store) ListInputLines(ctx context.Context, periodID, employeeID string) ([]InputLine, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT pe.element_type, pi.amount
    FROM payroll_inputs pi
    JOIN pay_elements pe ON pi.element_id = pe.id
    WHERE pi.period_id = $1 AND pi.employee_id = $2
  `, periodID, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var inputs []InputLine
	for rows.Next() {
		var line InputLine
		if err := rows.Scan(&line.Type, &line.Amount); err != nil {
			return nil, err
		}
		inputs = append(inputs, line)
	}
	return inputs, nil
}

func (s *Store) ListAdjustmentAmounts(ctx context.Context, tenantID, periodID, employeeID string, periodStart, periodEnd time.Time) ([]float64, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT amount
    FROM payroll_adjustments
    WHERE tenant_id = $1 AND period_id = $2 AND employee_id = $3
      AND (effective_date IS NULL OR (effective_date >= $4 AND effective_date <= $5))
  `, tenantID, periodID, employeeID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []float64
	for rows.Next() {
		var amount float64
		if err := rows.Scan(&amount); err != nil {
			continue
		}
		out = append(out, amount)
	}
	return out, nil
}

func (s *Store) ListUnpaidLeaves(ctx context.Context, tenantID, employeeID string, periodStart, periodEnd time.Time, status string) ([]LeaveWindow, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT lr.start_date, lr.end_date, lr.start_half, lr.end_half
    FROM leave_requests lr
    JOIN leave_types lt ON lr.leave_type_id = lt.id
    WHERE lr.tenant_id = $1
      AND lr.employee_id = $2
      AND lr.status = $3
      AND lt.is_paid = false
      AND lr.start_date <= $4
      AND lr.end_date >= $5
  `, tenantID, employeeID, status, periodEnd, periodStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LeaveWindow
	for rows.Next() {
		var window LeaveWindow
		if err := rows.Scan(&window.StartDate, &window.EndDate, &window.StartHalf, &window.EndHalf); err != nil {
			continue
		}
		out = append(out, window)
	}
	return out, nil
}

func (s *Store) LatestNet(ctx context.Context, tenantID, employeeID string) (float64, error) {
	var previousNet float64
	if err := s.DB.QueryRow(ctx, `
    SELECT net
    FROM payroll_results
    WHERE tenant_id = $1 AND employee_id = $2
    ORDER BY created_at DESC
    LIMIT 1
  `, tenantID, employeeID).Scan(&previousNet); err != nil {
		return 0, err
	}
	return previousNet, nil
}

func (s *Store) UpsertPayrollResult(ctx context.Context, tenantID, periodID, employeeID string, gross, deductions, net float64, currency string, warningsJSON []byte) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO payroll_results (tenant_id, period_id, employee_id, gross, deductions, net, currency, warnings_json)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    ON CONFLICT (period_id, employee_id)
    DO UPDATE SET gross = EXCLUDED.gross, deductions = EXCLUDED.deductions, net = EXCLUDED.net, warnings_json = EXCLUDED.warnings_json
  `, tenantID, periodID, employeeID, gross, deductions, net, currency, warningsJSON)
	return err
}

func (s *Store) UpdatePeriodStatus(ctx context.Context, tenantID, periodID, status string) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE payroll_periods SET status = $1 WHERE id = $2 AND tenant_id = $3
  `, status, periodID, tenantID)
	return err
}

func (s *Store) FinalizePeriod(ctx context.Context, tenantID, periodID string) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE payroll_periods SET status = $1, finalized_at = now() WHERE id = $2 AND tenant_id = $3
  `, PeriodStatusFinalized, periodID, tenantID)
	return err
}

func (s *Store) CreatePayslipsForPeriod(ctx context.Context, periodID string) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO payslips (tenant_id, period_id, employee_id)
    SELECT tenant_id, period_id, employee_id
    FROM payroll_results
    WHERE period_id = $1
    ON CONFLICT DO NOTHING
  `, periodID)
	return err
}

func (s *Store) ListPayslipIDs(ctx context.Context, tenantID, periodID string) ([]PayslipKey, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, employee_id
    FROM payslips
    WHERE tenant_id = $1 AND period_id = $2
  `, tenantID, periodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PayslipKey
	for rows.Next() {
		var key PayslipKey
		if err := rows.Scan(&key.ID, &key.EmployeeID); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, nil
}

func (s *Store) UpdatePayslipFileURL(ctx context.Context, payslipID, fileURL string) error {
	_, err := s.DB.Exec(ctx, "UPDATE payslips SET file_url = $1 WHERE id = $2", fileURL, payslipID)
	return err
}

func (s *Store) EmployeeUserID(ctx context.Context, tenantID, employeeID string) (string, error) {
	var userID string
	if err := s.DB.QueryRow(ctx, "SELECT user_id FROM employees WHERE tenant_id = $1 AND id = $2", tenantID, employeeID).Scan(&userID); err != nil {
		return "", err
	}
	return userID, nil
}

func (s *Store) CountPayslips(ctx context.Context, tenantID, employeeID string) (int, error) {
	var total int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM payslips WHERE tenant_id = $1 AND employee_id = $2", tenantID, employeeID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) ListPayslips(ctx context.Context, tenantID, employeeID string, limit, offset int) ([]Payslip, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT p.id, p.period_id, p.employee_id, r.gross, r.deductions, r.net, r.currency, p.file_url, p.created_at
    FROM payslips p
    JOIN payroll_results r ON p.period_id = r.period_id AND p.employee_id = r.employee_id
    WHERE p.tenant_id = $1 AND p.employee_id = $2
    ORDER BY p.created_at DESC
    LIMIT $3 OFFSET $4
  `, tenantID, employeeID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slips []Payslip
	for rows.Next() {
		var slip Payslip
		if err := rows.Scan(&slip.ID, &slip.PeriodID, &slip.EmployeeID, &slip.Gross, &slip.Deductions, &slip.Net, &slip.Currency, &slip.FileURL, &slip.CreatedAt); err != nil {
			return nil, err
		}
		slips = append(slips, slip)
	}
	return slips, nil
}

func (s *Store) ListAdjustments(ctx context.Context, tenantID, periodID, employeeID string, limit, offset int) ([]Adjustment, int, error) {
	query := `
    SELECT id, employee_id, description, amount, effective_date, created_at
    FROM payroll_adjustments
    WHERE tenant_id = $1 AND period_id = $2
  `
	args := []any{tenantID, periodID}
	if employeeID != "" {
		query += " AND employee_id = $3"
		args = append(args, employeeID)
	}
	query += " ORDER BY created_at DESC"

	countQuery := `
    SELECT COUNT(1)
    FROM payroll_adjustments
    WHERE tenant_id = $1 AND period_id = $2
  `
	countArgs := []any{tenantID, periodID}
	if employeeID != "" {
		countQuery += " AND employee_id = $3"
		countArgs = append(countArgs, employeeID)
	}
	var total int
	if err := s.DB.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		total = 0
	}

	query += " LIMIT $" + itoa(len(args)+1) + " OFFSET $" + itoa(len(args)+2)
	args = append(args, limit, offset)
	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var adjustments []Adjustment
	for rows.Next() {
		var adj Adjustment
		var effectiveDate *time.Time
		if err := rows.Scan(&adj.ID, &adj.EmployeeID, &adj.Description, &adj.Amount, &effectiveDate, &adj.CreatedAt); err != nil {
			return nil, 0, err
		}
		if effectiveDate != nil {
			adj.EffectiveDate = effectiveDate.Format("2006-01-02")
		}
		adjustments = append(adjustments, adj)
	}
	return adjustments, total, nil
}

func (s *Store) CreateAdjustment(ctx context.Context, tenantID, periodID, employeeID, description string, amount float64, effectiveDate any) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO payroll_adjustments (tenant_id, period_id, employee_id, description, amount, effective_date)
    VALUES ($1,$2,$3,$4,$5,$6)
    RETURNING id
  `, tenantID, periodID, employeeID, description, amount, effectiveDate).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) PeriodSummaryData(ctx context.Context, tenantID, periodID string) (PeriodSummary, [][]byte, error) {
	var summary PeriodSummary
	if err := s.DB.QueryRow(ctx, `
    SELECT COALESCE(SUM(gross),0), COALESCE(SUM(deductions),0), COALESCE(SUM(net),0), COUNT(1)
    FROM payroll_results
    WHERE tenant_id = $1 AND period_id = $2
  `, tenantID, periodID).Scan(&summary.TotalGross, &summary.TotalDeductions, &summary.TotalNet, &summary.EmployeeCount); err != nil {
		return summary, nil, err
	}

	rows, err := s.DB.Query(ctx, `
    SELECT warnings_json
    FROM payroll_results
    WHERE tenant_id = $1 AND period_id = $2
  `, tenantID, periodID)
	if err != nil {
		return summary, nil, nil
	}
	defer rows.Close()
	var warnings [][]byte
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		warnings = append(warnings, raw)
	}
	return summary, warnings, nil
}

func (s *Store) PeriodStatus(ctx context.Context, tenantID, periodID string) (string, error) {
	var status string
	if err := s.DB.QueryRow(ctx, `
    SELECT status
    FROM payroll_periods
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, periodID).Scan(&status); err != nil {
		return "", err
	}
	return status, nil
}

func (s *Store) DeleteResultsForPeriod(ctx context.Context, tenantID, periodID string) error {
	_, err := s.DB.Exec(ctx, "DELETE FROM payroll_results WHERE tenant_id = $1 AND period_id = $2", tenantID, periodID)
	return err
}

func (s *Store) DeletePayslipsForPeriod(ctx context.Context, tenantID, periodID string) error {
	_, err := s.DB.Exec(ctx, "DELETE FROM payslips WHERE tenant_id = $1 AND period_id = $2", tenantID, periodID)
	return err
}

func (s *Store) RegisterRows(ctx context.Context, tenantID, periodID string) ([]RegisterRow, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT e.id, e.first_name, e.last_name, r.gross, r.deductions, r.net, r.currency
    FROM payroll_results r
    JOIN employees e ON r.employee_id = e.id
    WHERE r.tenant_id = $1 AND r.period_id = $2
    ORDER BY e.last_name, e.first_name
  `, tenantID, periodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RegisterRow
	for rows.Next() {
		var row RegisterRow
		if err := rows.Scan(&row.EmployeeID, &row.FirstName, &row.LastName, &row.Gross, &row.Deductions, &row.Net, &row.Currency); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Store) PeriodTotals(ctx context.Context, tenantID, periodID string) (float64, float64, float64, error) {
	var gross, deductions, net float64
	if err := s.DB.QueryRow(ctx, `
    SELECT COALESCE(SUM(gross),0), COALESCE(SUM(deductions),0), COALESCE(SUM(net),0)
    FROM payroll_results
    WHERE tenant_id = $1 AND period_id = $2
  `, tenantID, periodID).Scan(&gross, &deductions, &net); err != nil {
		return 0, 0, 0, err
	}
	return gross, deductions, net, nil
}

func (s *Store) JournalTemplateConfig(ctx context.Context, tenantID, templateID string) ([]byte, error) {
	var raw []byte
	if err := s.DB.QueryRow(ctx, `
    SELECT config_json
    FROM journal_templates
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, templateID).Scan(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Store) PayslipInfo(ctx context.Context, tenantID, payslipID string) (string, string, error) {
	var employeeID, fileURL string
	if err := s.DB.QueryRow(ctx, `
    SELECT employee_id, COALESCE(file_url, '')
    FROM payslips
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, payslipID).Scan(&employeeID, &fileURL); err != nil {
		return "", "", err
	}
	return employeeID, fileURL, nil
}

func (s *Store) PayslipPeriodID(ctx context.Context, tenantID, payslipID string) (string, error) {
	var periodID string
	if err := s.DB.QueryRow(ctx, "SELECT period_id FROM payslips WHERE tenant_id = $1 AND id = $2", tenantID, payslipID).Scan(&periodID); err != nil {
		return "", err
	}
	return periodID, nil
}

func (s *Store) PayslipEmployeePeriod(ctx context.Context, tenantID, payslipID string) (string, string, error) {
	var employeeID, periodID string
	if err := s.DB.QueryRow(ctx, `
    SELECT employee_id, period_id
    FROM payslips
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, payslipID).Scan(&employeeID, &periodID); err != nil {
		return "", "", err
	}
	return employeeID, periodID, nil
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
