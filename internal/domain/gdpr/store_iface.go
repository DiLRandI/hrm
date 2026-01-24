package gdpr

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type StoreAPI interface {
	ListRetentionPolicies(ctx context.Context, tenantID string) ([]RetentionPolicy, error)
	UpsertRetentionPolicy(ctx context.Context, tenantID, dataCategory string, retentionDays int) (string, error)
	ListRetentionRuns(ctx context.Context, tenantID string) ([]RetentionRun, error)
	CreateJobRun(ctx context.Context, tenantID, jobType string) (string, error)
	UpdateJobRun(ctx context.Context, runID, status string, detailsJSON []byte) error
	RecordRetentionRun(ctx context.Context, tenantID, category string, cutoff time.Time, status string, deleted int64) (string, error)
	ListConsents(ctx context.Context, tenantID, employeeID string) ([]ConsentRecord, error)
	CreateConsent(ctx context.Context, tenantID, employeeID, consentType string) (string, error)
	RevokeConsent(ctx context.Context, tenantID, consentID string) error
	ListDSARExports(ctx context.Context, tenantID, employeeID string, limit, offset int) ([]DSARExport, int, error)
	CreateDSARExport(ctx context.Context, tenantID, employeeID, requestedBy, status string) (string, error)
	UpdateDSARExport(ctx context.Context, exportID, status, filePath string, encrypted bool, token string, expiresAt time.Time) error
	UpdateDSARStatus(ctx context.Context, exportID, status string) error
	DSARDownloadInfo(ctx context.Context, tenantID, exportID string) (string, string, string, bool, any, error)
	ListAnonymizationJobs(ctx context.Context, tenantID string) ([]AnonymizationJob, error)
	CreateAnonymizationJob(ctx context.Context, tenantID, employeeID, status, reason string) (string, error)
	AnonymizationJobStatus(ctx context.Context, tenantID, jobID string) (string, string, error)
	UpdateAnonymizationStatus(ctx context.Context, tenantID, jobID, status string) error
	UpdateAnonymizationReport(ctx context.Context, tenantID, jobID, filePath, token string, expiresAt time.Time) error
	AnonymizationReportInfo(ctx context.Context, tenantID, jobID string) (string, string, any, error)
	ListAccessLogs(ctx context.Context, tenantID string, limit, offset int) ([]AccessLog, int, error)
	EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error)
	ApplyRetention(ctx context.Context, tenantID, category string, cutoff time.Time) (int64, error)
	DSARLeaveRequests(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARPayrollResults(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARPayslips(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARGoals(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARGoalComments(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARFeedback(ctx context.Context, tenantID, employeeID, userID string) ([]map[string]any, error)
	DSARCheckins(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARPIPs(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARReviewTasks(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARReviewResponses(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARConsentRecords(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARNotifications(ctx context.Context, tenantID, userID string) ([]map[string]any, error)
	DSARAccessLogs(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	DSARManagerHistory(ctx context.Context, employeeID string) ([]map[string]any, error)
	DSAREmergencyContacts(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
	UpdateAnonymizationStatusTx(ctx context.Context, tx pgx.Tx, tenantID, jobID, status string) error
	EmployeeUserIDTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) (string, error)
	AnonymizeEmployeeTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID, email, status string) error
	AnonymizeUserTx(ctx context.Context, tx pgx.Tx, tenantID, userID, email, status string) error
	AnonymizeLeaveRequestsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error
	AnonymizeGoalsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error
	AnonymizeFeedbackTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error
	AnonymizeCheckinsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error
	AnonymizePIPsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error
	ClearPayslipURLsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error
	DeleteEmergencyContactsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error
	CompleteAnonymizationJobTx(ctx context.Context, tx pgx.Tx, tenantID, jobID, status string) error
}
