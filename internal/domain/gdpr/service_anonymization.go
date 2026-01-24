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

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return AnonymizationResult{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.store.UpdateAnonymizationStatusTx(ctx, tx, tenantID, jobID, AnonymizationProcessing); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	userID, err := s.store.EmployeeUserIDTx(ctx, tx, tenantID, employeeID)
	if err != nil {
		slog.Warn("anonymization user lookup failed", "err", err)
	}

	anonEmployeeEmail := fmt.Sprintf("anonymized+%s@example.local", employeeID)
	if err := s.store.AnonymizeEmployeeTx(ctx, tx, tenantID, employeeID, anonEmployeeEmail, core.EmployeeStatusAnonymized); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if userID != "" {
		anonUserEmail := fmt.Sprintf("anonymized+%s@example.local", userID)
		if err := s.store.AnonymizeUserTx(ctx, tx, tenantID, userID, anonUserEmail, core.UserStatusDisabled); err != nil {
			s.failAnonymization(ctx, tenantID, jobID)
			return AnonymizationResult{}, err
		}
	}

	if err := s.store.AnonymizeLeaveRequestsTx(ctx, tx, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := s.store.AnonymizeGoalsTx(ctx, tx, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := s.store.AnonymizeFeedbackTx(ctx, tx, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := s.store.AnonymizeCheckinsTx(ctx, tx, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := s.store.AnonymizePIPsTx(ctx, tx, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := s.store.ClearPayslipURLsTx(ctx, tx, tenantID, employeeID); err != nil {
		s.failAnonymization(ctx, tenantID, jobID)
		return AnonymizationResult{}, err
	}

	if err := s.store.CompleteAnonymizationJobTx(ctx, tx, tenantID, jobID, AnonymizationCompleted); err != nil {
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
