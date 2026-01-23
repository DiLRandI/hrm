package payroll

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jung-kurt/gofpdf"

	cryptoutil "hrm/internal/platform/crypto"
)

type Service struct {
	store  *Store
	crypto *cryptoutil.Service
}

func NewService(store *Store, crypto *cryptoutil.Service) *Service {
	return &Service{store: store, crypto: crypto}
}

func (s *Service) Pool() *pgxpool.Pool {
	return s.store.DB
}

func (s *Service) GeneratePayslipPDF(ctx context.Context, tenantID, periodID, employeeID, payslipID string) (string, error) {
	var firstName, lastName, email, currency string
	var gross, deductions, net float64
	var startDate, endDate time.Time
	err := s.store.DB.QueryRow(ctx, `
    SELECT e.first_name, e.last_name, e.email,
           r.gross, r.deductions, r.net, r.currency,
           p.start_date, p.end_date
    FROM payroll_results r
    JOIN employees e ON r.employee_id = e.id
    JOIN payroll_periods p ON r.period_id = p.id
    WHERE r.tenant_id = $1 AND r.period_id = $2 AND r.employee_id = $3
  `, tenantID, periodID, employeeID).Scan(&firstName, &lastName, &email, &gross, &deductions, &net, &currency, &startDate, &endDate)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll("storage/payslips", 0o755); err != nil {
		return "", err
	}
	filePath := filepath.Join("storage/payslips", payslipID+".pdf")

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(40, 10, "Payslip")
	pdf.Ln(12)
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(0, 8, fmt.Sprintf("Employee: %s %s", firstName, lastName))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Email: %s", email))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Period: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")))
	pdf.Ln(10)
	pdf.Cell(0, 8, fmt.Sprintf("Gross: %.2f %s", gross, currency))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Deductions: %.2f %s", deductions, currency))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Net: %.2f %s", net, currency))

	if err := pdf.OutputFileAndClose(filePath); err != nil {
		return "", err
	}

	if s.crypto != nil && s.crypto.Configured() {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		encrypted, err := s.crypto.Encrypt(data)
		if err != nil {
			return "", err
		}
		encryptedPath := filePath + ".enc"
		if err := os.WriteFile(encryptedPath, encrypted, 0o600); err != nil {
			return "", err
		}
		if err := os.Remove(filePath); err != nil {
			return "", err
		}
		return encryptedPath, nil
	}

	return filePath, nil
}
