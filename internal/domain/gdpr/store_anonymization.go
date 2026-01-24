package gdpr

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (s *Store) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return s.DB.Begin(ctx)
}

func (s *Store) UpdateAnonymizationStatusTx(ctx context.Context, tx pgx.Tx, tenantID, jobID, status string) error {
	_, err := tx.Exec(ctx, `
    UPDATE anonymization_jobs
    SET status = $1
    WHERE tenant_id = $2 AND id = $3
  `, status, tenantID, jobID)
	return err
}

func (s *Store) EmployeeUserIDTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) (string, error) {
	var userID string
	if err := tx.QueryRow(ctx, `
    SELECT user_id
    FROM employees
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, employeeID).Scan(&userID); err != nil {
		return "", err
	}
	return userID, nil
}

func (s *Store) AnonymizeEmployeeTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID, email, status string) error {
	_, err := tx.Exec(ctx, `
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
  `, email, status, tenantID, employeeID)
	return err
}

func (s *Store) AnonymizeUserTx(ctx context.Context, tx pgx.Tx, tenantID, userID, email, status string) error {
	_, err := tx.Exec(ctx, `
      UPDATE users
      SET email = $1,
          status = $2,
          mfa_secret_enc = NULL,
          mfa_enabled = false,
          updated_at = now()
      WHERE tenant_id = $3 AND id = $4
    `, email, status, tenantID, userID)
	return err
}

func (s *Store) AnonymizeLeaveRequestsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error {
	_, err := tx.Exec(ctx, `
    UPDATE leave_requests
    SET reason = NULL
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	return err
}

func (s *Store) AnonymizeGoalsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error {
	_, err := tx.Exec(ctx, `
    UPDATE goals
    SET title = 'Anonymized goal', description = NULL, metric = NULL, updated_at = now()
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	return err
}

func (s *Store) AnonymizeFeedbackTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error {
	_, err := tx.Exec(ctx, `
    UPDATE feedback
    SET message = 'Anonymized'
    WHERE tenant_id = $1 AND to_employee_id = $2
  `, tenantID, employeeID)
	return err
}

func (s *Store) AnonymizeCheckinsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error {
	_, err := tx.Exec(ctx, `
    UPDATE checkins
    SET notes = 'Anonymized', private = true
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	return err
}

func (s *Store) AnonymizePIPsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error {
	_, err := tx.Exec(ctx, `
    UPDATE pips
    SET objectives_json = NULL, milestones_json = NULL, review_dates_json = NULL, updated_at = now()
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	return err
}

func (s *Store) ClearPayslipURLsTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID string) error {
	_, err := tx.Exec(ctx, `
    UPDATE payslips
    SET file_url = NULL
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	return err
}

func (s *Store) CompleteAnonymizationJobTx(ctx context.Context, tx pgx.Tx, tenantID, jobID, status string) error {
	_, err := tx.Exec(ctx, `
    UPDATE anonymization_jobs
    SET status = $1, completed_at = now()
    WHERE tenant_id = $2 AND id = $3
  `, status, tenantID, jobID)
	return err
}
