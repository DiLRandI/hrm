package payroll

import (
	"context"
	"time"
)

type PayslipPDFData struct {
	FirstName  string
	LastName   string
	Email      string
	Gross      float64
	Deductions float64
	Net        float64
	Currency   string
	StartDate  time.Time
	EndDate    time.Time
}

func (s *Store) PayslipPDFData(ctx context.Context, tenantID, periodID, employeeID string) (PayslipPDFData, error) {
	var data PayslipPDFData
	err := s.DB.QueryRow(ctx, `
    SELECT e.first_name, e.last_name, e.email,
           r.gross, r.deductions, r.net, r.currency,
           p.start_date, p.end_date
    FROM payroll_results r
    JOIN employees e ON r.employee_id = e.id
    JOIN payroll_periods p ON r.period_id = p.id
    WHERE r.tenant_id = $1 AND r.period_id = $2 AND r.employee_id = $3
  `, tenantID, periodID, employeeID).Scan(&data.FirstName, &data.LastName, &data.Email, &data.Gross, &data.Deductions, &data.Net, &data.Currency, &data.StartDate, &data.EndDate)
	if err != nil {
		return PayslipPDFData{}, err
	}
	return data, nil
}
