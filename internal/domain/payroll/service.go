package payroll

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jung-kurt/gofpdf"

	cryptoutil "hrm/internal/platform/crypto"
)

type Service struct {
	Store  *Store
	Crypto *cryptoutil.Service
}

func NewService(store *Store, crypto *cryptoutil.Service) *Service {
	return &Service{Store: store, Crypto: crypto}
}

func (s *Service) Pool() *pgxpool.Pool {
	return s.Store.DB
}

func (s *Service) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return s.Store.DB.Query(ctx, sql, args...)
}

func (s *Service) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return s.Store.DB.QueryRow(ctx, sql, args...)
}

func (s *Service) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return s.Store.DB.Exec(ctx, sql, args...)
}

func (s *Service) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.Store.DB.Begin(ctx)
}

func (s *Service) GeneratePayslipPDF(ctx context.Context, tenantID, periodID, employeeID, payslipID string) (string, error) {
	var firstName, lastName, email, currency string
	var gross, deductions, net float64
	var startDate, endDate time.Time
	err := s.Store.DB.QueryRow(ctx, `
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

	if s.Crypto != nil && s.Crypto.Configured() {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		encrypted, err := s.Crypto.Encrypt(data)
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
