package payroll

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

func (s *Service) GeneratePayslipPDF(ctx context.Context, tenantID, periodID, employeeID, payslipID string) (string, error) {
	data, err := s.store.PayslipPDFData(ctx, tenantID, periodID, employeeID)
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
	pdf.Cell(0, 8, fmt.Sprintf("Employee: %s %s", data.FirstName, data.LastName))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Email: %s", data.Email))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Period: %s to %s", data.StartDate.Format("2006-01-02"), data.EndDate.Format("2006-01-02")))
	pdf.Ln(10)
	pdf.Cell(0, 8, fmt.Sprintf("Gross: %.2f %s", data.Gross, data.Currency))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Deductions: %.2f %s", data.Deductions, data.Currency))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Net: %.2f %s", data.Net, data.Currency))

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
