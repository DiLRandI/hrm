package gdpr

import (
	"context"
	"encoding/json"
	"time"
)

func (s *Service) ListRetentionPolicies(ctx context.Context, tenantID string) ([]RetentionPolicy, error) {
	return s.store.ListRetentionPolicies(ctx, tenantID)
}

func (s *Service) UpsertRetentionPolicy(ctx context.Context, tenantID, dataCategory string, retentionDays int) (string, error) {
	return s.store.UpsertRetentionPolicy(ctx, tenantID, dataCategory, retentionDays)
}

func (s *Service) ListRetentionRuns(ctx context.Context, tenantID string) ([]RetentionRun, error) {
	return s.store.ListRetentionRuns(ctx, tenantID)
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

func (s *Service) RecordRetentionRun(ctx context.Context, tenantID, category string, cutoff time.Time, status string, deleted int64) (string, error) {
	return s.store.RecordRetentionRun(ctx, tenantID, category, cutoff, status, deleted)
}

func (s *Service) ListConsents(ctx context.Context, tenantID, employeeID string) ([]ConsentRecord, error) {
	return s.store.ListConsents(ctx, tenantID, employeeID)
}

func (s *Service) CreateConsent(ctx context.Context, tenantID, employeeID, consentType string) (string, error) {
	return s.store.CreateConsent(ctx, tenantID, employeeID, consentType)
}

func (s *Service) RevokeConsent(ctx context.Context, tenantID, consentID string) error {
	return s.store.RevokeConsent(ctx, tenantID, consentID)
}

func (s *Service) ListDSARExports(ctx context.Context, tenantID, employeeID string, limit, offset int) ([]DSARExport, int, error) {
	return s.store.ListDSARExports(ctx, tenantID, employeeID, limit, offset)
}

func (s *Service) CreateDSARExport(ctx context.Context, tenantID, employeeID, requestedBy, status string) (string, error) {
	return s.store.CreateDSARExport(ctx, tenantID, employeeID, requestedBy, status)
}

func (s *Service) UpdateDSARExport(ctx context.Context, exportID, status, filePath string, encrypted bool, token string, expiresAt time.Time) error {
	return s.store.UpdateDSARExport(ctx, exportID, status, filePath, encrypted, token, expiresAt)
}

func (s *Service) UpdateDSARStatus(ctx context.Context, exportID, status string) error {
	return s.store.UpdateDSARStatus(ctx, exportID, status)
}

func (s *Service) DSARDownloadInfo(ctx context.Context, tenantID, exportID string) (string, string, string, bool, any, error) {
	return s.store.DSARDownloadInfo(ctx, tenantID, exportID)
}

func (s *Service) ListAnonymizationJobs(ctx context.Context, tenantID string) ([]AnonymizationJob, error) {
	return s.store.ListAnonymizationJobs(ctx, tenantID)
}

func (s *Service) CreateAnonymizationJob(ctx context.Context, tenantID, employeeID, status, reason string) (string, error) {
	return s.store.CreateAnonymizationJob(ctx, tenantID, employeeID, status, reason)
}

func (s *Service) AnonymizationJobStatus(ctx context.Context, tenantID, jobID string) (string, string, error) {
	return s.store.AnonymizationJobStatus(ctx, tenantID, jobID)
}

func (s *Service) UpdateAnonymizationStatus(ctx context.Context, tenantID, jobID, status string) error {
	return s.store.UpdateAnonymizationStatus(ctx, tenantID, jobID, status)
}

func (s *Service) UpdateAnonymizationReport(ctx context.Context, tenantID, jobID, filePath, token string, expiresAt time.Time) error {
	return s.store.UpdateAnonymizationReport(ctx, tenantID, jobID, filePath, token, expiresAt)
}

func (s *Service) AnonymizationReportInfo(ctx context.Context, tenantID, jobID string) (string, string, any, error) {
	return s.store.AnonymizationReportInfo(ctx, tenantID, jobID)
}

func (s *Service) ListAccessLogs(ctx context.Context, tenantID string, limit, offset int) ([]AccessLog, int, error) {
	return s.store.ListAccessLogs(ctx, tenantID, limit, offset)
}

func (s *Service) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	return s.store.EmployeeIDByUserID(ctx, tenantID, userID)
}
