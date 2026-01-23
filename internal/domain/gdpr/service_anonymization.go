package gdpr

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"hrm/internal/domain/core"
)

type AnonymizationResult struct {
	EmployeeID string
}

func (s *Service) ExecuteAnonymization(ctx context.Context, tenantID, jobID string) (AnonymizationResult, error) {
	employeeID, status, err := s.AnonymizationJobStatus(ctx, tenantID, jobID)
	if err != nil {
		return AnonymizationResult{}, ErrAnonymizationNotFound
	}
	if status != AnonymizationRequested {
		return AnonymizationResult{}, ErrAnonymizationBadState
	}

	tx, err := s.store.DB.Begin(ctx)
	if err != nil {
		return AnonymizationResult{}, err
	}
	defer tx.Rollback(ctx)

	execTx := func(query string, args ...any) error {
		_, execErr := tx.Exec(ctx, query, args...)
		return execErr
	}

	if err := execTx(`
    UPDATE anonymization_jobs
    SET status = $1
    WHERE tenant_id = $2 AND id = $3
  `, AnonymizationProcessing, tenantID, jobID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	var userID string
	if err := tx.QueryRow(ctx, `
    SELECT user_id
    FROM employees
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, employeeID).Scan(&userID); err != nil {
		slog.Warn("anonymization user lookup failed", "err", err)
	}

	anonEmployeeEmail := fmt.Sprintf("anonymized+%s@example.local", employeeID)
	if err := execTx(`
    UPDATE employees
    SET employee_number = NULL,
        first_name = 'Anonymized',
        last_name = 'Employee',
        email = $1,
        phone = NULL,
        address = NULL,
        national_id = NULL,
        national_id_enc = NULL,
        bank_account = NULL,
        bank_account_enc = NULL,
        salary = NULL,
        salary_enc = NULL,
        currency = COALESCE(currency, 'USD'),
        employment_type = NULL,
        department_id = NULL,
        manager_id = NULL,
        pay_group_id = NULL,
        status = $2,
        updated_at = now()
    WHERE tenant_id = $3 AND id = $4
  `, anonEmployeeEmail, core.EmployeeStatusAnonymized, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if userID != "" {
		anonUserEmail := fmt.Sprintf("anonymized+%s@example.local", userID)
		if err := execTx(`
      UPDATE users
      SET email = $1,
          status = $2,
          mfa_secret_enc = NULL,
          mfa_enabled = false,
          updated_at = now()
      WHERE tenant_id = $3 AND id = $4
    `, anonUserEmail, core.UserStatusDisabled, tenantID, userID); err != nil {
			s.failAnonymization(ctx, tenantID, jobID)
			return AnonymizationResult{}, err
		}
	}

	if err := execTx(`
    UPDATE leave_requests
    SET reason = NULL
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := execTx(`
    UPDATE goals
    SET title = 'Anonymized goal', description = NULL, metric = NULL, updated_at = now()
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := execTx(`
    UPDATE feedback
    SET message = 'Anonymized'
    WHERE tenant_id = $1 AND to_employee_id = $2
  `, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := execTx(`
    UPDATE checkins
    SET notes = 'Anonymized', private = true
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := execTx(`
    UPDATE pips
    SET objectives_json = NULL, milestones_json = NULL, review_dates_json = NULL, updated_at = now()
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := execTx(`
    UPDATE payslips
    SET file_url = NULL
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := execTx(`
    UPDATE anonymization_jobs
    SET status = $1, completed_at = now()
    WHERE tenant_id = $2 AND id = $3
  `, AnonymizationCompleted, tenantID, jobID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	s.writeAnonymizationReport(ctx, tenantID, jobID, employeeID)
	return AnonymizationResult{EmployeeID: employeeID}, nil
}

func (s *Service) writeAnonymizationReport(ctx context.Context, tenantID, jobID, employeeID string) {
	report := map[string]any{
		"employeeId":  employeeID,
		"status":      AnonymizationCompleted,
		"completedAt": time.Now(),
	}
	if err := os.MkdirAll("storage/anonymization", 0o755); err != nil {
		return
	}
	reportPath := filepath.Join("storage/anonymization", jobID+".json")
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		slog.Warn("anonymization report marshal failed", "err", err)
		reportBytes = []byte("{}")
	}
	if s.crypto != nil && s.crypto.Configured() {
		if enc, err := s.crypto.Encrypt(reportBytes); err == nil {
			reportPath = reportPath + ".enc"
			if err := os.WriteFile(reportPath, enc, 0o600); err == nil {
				if token, tokenErr := generateDownloadToken(); tokenErr == nil {
					expiresAt := time.Now().Add(24 * time.Hour)
					if err := s.UpdateAnonymizationReport(ctx, tenantID, jobID, reportPath, token, expiresAt); err != nil {
						slog.Warn("anonymization report update failed", "err", err)
					}
				}
			}
			return
		}
	}
	if err := os.WriteFile(reportPath, reportBytes, 0o600); err == nil {
		if token, tokenErr := generateDownloadToken(); tokenErr == nil {
			expiresAt := time.Now().Add(24 * time.Hour)
			if err := s.UpdateAnonymizationReport(ctx, tenantID, jobID, reportPath, token, expiresAt); err != nil {
				slog.Warn("anonymization report update failed", "err", err)
			}
		}
	}
}

func (s *Service) failAnonymization(ctx context.Context, tenantID, jobID string) {
	if err := s.UpdateAnonymizationStatus(ctx, tenantID, jobID, AnonymizationFailed); err != nil {
		slog.Warn("anonymization fail update failed", "err", err)
	}
}
