package gdpr

import (
	"context"
	"fmt"
	"time"
)

func (s *Store) ListRetentionPolicies(ctx context.Context, tenantID string) ([]RetentionPolicy, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, data_category, retention_days
    FROM retention_policies
    WHERE tenant_id = $1
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []RetentionPolicy
	for rows.Next() {
		var policy RetentionPolicy
		if err := rows.Scan(&policy.ID, &policy.DataCategory, &policy.RetentionDays); err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, nil
}

func (s *Store) ApplyRetention(ctx context.Context, tenantID, category string, cutoff time.Time) (int64, error) {
	return ApplyRetention(ctx, s.DB, tenantID, category, cutoff)
}

func (s *Store) UpsertRetentionPolicy(ctx context.Context, tenantID, dataCategory string, retentionDays int) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO retention_policies (tenant_id, data_category, retention_days)
    VALUES ($1,$2,$3)
    ON CONFLICT (tenant_id, data_category) DO UPDATE SET retention_days = EXCLUDED.retention_days
    RETURNING id
  `, tenantID, dataCategory, retentionDays).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) ListRetentionRuns(ctx context.Context, tenantID string) ([]RetentionRun, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, data_category, cutoff_date, status, deleted_count, started_at, completed_at
    FROM retention_runs
    WHERE tenant_id = $1
    ORDER BY started_at DESC
    LIMIT 100
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []RetentionRun
	for rows.Next() {
		var run RetentionRun
		if err := rows.Scan(&run.ID, &run.DataCategory, &run.CutoffDate, &run.Status, &run.DeletedCount, &run.StartedAt, &run.CompletedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
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

func (s *Store) RecordRetentionRun(ctx context.Context, tenantID, category string, cutoff time.Time, status string, deleted int64) (string, error) {
	var runID string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO retention_runs (tenant_id, data_category, cutoff_date, status, deleted_count)
    VALUES ($1,$2,$3,$4,$5)
    RETURNING id
  `, tenantID, category, cutoff, status, deleted).Scan(&runID); err != nil {
		return "", err
	}
	return runID, nil
}

func (s *Store) ListConsents(ctx context.Context, tenantID, employeeID string) ([]ConsentRecord, error) {
	query := `
    SELECT id, employee_id, consent_type, granted_at, revoked_at
    FROM consent_records
    WHERE tenant_id = $1
  `
	args := []any{tenantID}
	if employeeID != "" {
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	query += " ORDER BY granted_at DESC"

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ConsentRecord
	for rows.Next() {
		var rec ConsentRecord
		if err := rows.Scan(&rec.ID, &rec.EmployeeID, &rec.ConsentType, &rec.GrantedAt, &rec.RevokedAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

func (s *Store) CreateConsent(ctx context.Context, tenantID, employeeID, consentType string) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO consent_records (tenant_id, employee_id, consent_type)
    VALUES ($1,$2,$3)
    RETURNING id
  `, tenantID, employeeID, consentType).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) RevokeConsent(ctx context.Context, tenantID, consentID string) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE consent_records SET revoked_at = now()
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, consentID)
	return err
}

func (s *Store) ListDSARExports(ctx context.Context, tenantID, employeeID string, limit, offset int) ([]DSARExport, int, error) {
	query := `
    SELECT id, employee_id, requested_by, status, COALESCE(file_path,''), COALESCE(download_token,''), requested_at, completed_at
    FROM dsar_exports
    WHERE tenant_id = $1
  `
	args := []any{tenantID}
	if employeeID != "" {
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	query += " ORDER BY requested_at DESC"

	countQuery := "SELECT COUNT(1) FROM dsar_exports WHERE tenant_id = $1"
	countArgs := []any{tenantID}
	if employeeID != "" {
		countQuery += " AND employee_id = $2"
		countArgs = append(countArgs, employeeID)
	}
	var total int
	if err := s.DB.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		total = 0
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", limitPos, offsetPos)
	args = append(args, limit, offset)

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var exports []DSARExport
	for rows.Next() {
		var export DSARExport
		if err := rows.Scan(&export.ID, &export.EmployeeID, &export.RequestedBy, &export.Status, &export.FilePath, &export.DownloadToken, &export.RequestedAt, &export.CompletedAt); err != nil {
			return nil, 0, err
		}
		exports = append(exports, export)
	}
	return exports, total, nil
}

func (s *Store) CreateDSARExport(ctx context.Context, tenantID, employeeID, requestedBy, status string) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO dsar_exports (tenant_id, employee_id, requested_by, status)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, employeeID, requestedBy, status).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) UpdateDSARExport(ctx context.Context, exportID, status, filePath string, encrypted bool, token string, expiresAt time.Time) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE dsar_exports
    SET status = $1,
        file_path = $2,
        file_encrypted = $3,
        download_token = $4,
        download_expires_at = $5,
        completed_at = now()
    WHERE id = $6
  `, status, filePath, encrypted, token, expiresAt, exportID)
	return err
}

func (s *Store) UpdateDSARStatus(ctx context.Context, exportID, status string) error {
	_, err := s.DB.Exec(ctx, "UPDATE dsar_exports SET status = $1 WHERE id = $2", status, exportID)
	return err
}

func (s *Store) DSARDownloadInfo(ctx context.Context, tenantID, exportID string) (string, string, string, bool, any, error) {
	var employeeID, filePath, token string
	var encrypted bool
	var tokenExpiry any
	if err := s.DB.QueryRow(ctx, `
    SELECT employee_id, COALESCE(file_path, ''), COALESCE(download_token,''), file_encrypted, download_expires_at
    FROM dsar_exports
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, exportID).Scan(&employeeID, &filePath, &token, &encrypted, &tokenExpiry); err != nil {
		return "", "", "", false, nil, err
	}
	return employeeID, filePath, token, encrypted, tokenExpiry, nil
}

func (s *Store) ListAnonymizationJobs(ctx context.Context, tenantID string) ([]AnonymizationJob, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, employee_id, status, reason, requested_at, completed_at, COALESCE(file_path,''), COALESCE(download_token,'')
    FROM anonymization_jobs
    WHERE tenant_id = $1
    ORDER BY requested_at DESC
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []AnonymizationJob
	for rows.Next() {
		var job AnonymizationJob
		if err := rows.Scan(&job.ID, &job.EmployeeID, &job.Status, &job.Reason, &job.RequestedAt, &job.CompletedAt, &job.FilePath, &job.DownloadToken); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (s *Store) CreateAnonymizationJob(ctx context.Context, tenantID, employeeID, status, reason string) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO anonymization_jobs (tenant_id, employee_id, status, reason)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, employeeID, status, reason).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) AnonymizationJobStatus(ctx context.Context, tenantID, jobID string) (string, string, error) {
	var employeeID, status string
	if err := s.DB.QueryRow(ctx, `
    SELECT employee_id, status
    FROM anonymization_jobs
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, jobID).Scan(&employeeID, &status); err != nil {
		return "", "", err
	}
	return employeeID, status, nil
}

func (s *Store) UpdateAnonymizationStatus(ctx context.Context, tenantID, jobID, status string) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE anonymization_jobs
    SET status = $1
    WHERE tenant_id = $2 AND id = $3
  `, status, tenantID, jobID)
	return err
}

func (s *Store) UpdateAnonymizationReport(ctx context.Context, tenantID, jobID, filePath, token string, expiresAt time.Time) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE anonymization_jobs
    SET file_path = $1, download_token = $2, download_expires_at = $3
    WHERE tenant_id = $4 AND id = $5
  `, filePath, token, expiresAt, tenantID, jobID)
	return err
}

func (s *Store) AnonymizationReportInfo(ctx context.Context, tenantID, jobID string) (string, string, any, error) {
	var filePath, token string
	var tokenExpiry any
	if err := s.DB.QueryRow(ctx, `
    SELECT COALESCE(file_path,''), COALESCE(download_token,''), download_expires_at
    FROM anonymization_jobs
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, jobID).Scan(&filePath, &token, &tokenExpiry); err != nil {
		return "", "", nil, err
	}
	return filePath, token, tokenExpiry, nil
}

func (s *Store) ListAccessLogs(ctx context.Context, tenantID string, limit, offset int) ([]AccessLog, int, error) {
	var total int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM access_logs WHERE tenant_id = $1", tenantID).Scan(&total); err != nil {
		total = 0
	}

	rows, err := s.DB.Query(ctx, `
    SELECT id, actor_user_id, employee_id, fields, request_id, created_at
    FROM access_logs
    WHERE tenant_id = $1
    ORDER BY created_at DESC
    LIMIT $2 OFFSET $3
  `, tenantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []AccessLog
	for rows.Next() {
		var logEntry AccessLog
		if err := rows.Scan(&logEntry.ID, &logEntry.ActorID, &logEntry.EmployeeID, &logEntry.Fields, &logEntry.RequestID, &logEntry.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, logEntry)
	}
	return logs, total, nil
}

func (s *Store) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	var employeeID string
	if err := s.DB.QueryRow(ctx, "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", tenantID, userID).Scan(&employeeID); err != nil {
		return "", err
	}
	return employeeID, nil
}
