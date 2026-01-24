package leave

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListAccrualPolicies(ctx context.Context, tenantID string) ([]policyRow, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, leave_type_id, accrual_rate, accrual_period, entitlement, carry_over_limit
    FROM leave_policies
    WHERE tenant_id = $1 AND accrual_rate IS NOT NULL
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := make([]policyRow, 0)
	for rows.Next() {
		var p policyRow
		if err := rows.Scan(&p.ID, &p.LeaveTypeID, &p.AccrualRate, &p.AccrualPeriod, &p.Entitlement, &p.CarryOver); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, nil
}

func (s *Store) LastAccruedOn(ctx context.Context, tenantID, policyID string) (time.Time, error) {
	var last time.Time
	err := s.DB.QueryRow(ctx, `
    SELECT last_accrued_on
    FROM leave_accrual_runs
    WHERE tenant_id = $1 AND policy_id = $2
  `, tenantID, policyID).Scan(&last)
	return last, err
}

func (s *Store) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return s.DB.Begin(ctx)
}

func (s *Store) ListActiveEmployeesTx(ctx context.Context, tx pgx.Tx, tenantID string) (map[string]*time.Time, error) {
	rows, err := tx.Query(ctx, `
    SELECT id, start_date
    FROM employees
    WHERE tenant_id = $1 AND status = 'active'
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	employees := make(map[string]*time.Time)
	for rows.Next() {
		var employeeID string
		var startDate *time.Time
		if err := rows.Scan(&employeeID, &startDate); err != nil {
			return nil, err
		}
		employees[employeeID] = startDate
	}
	return employees, nil
}

func (s *Store) UpsertBalanceTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID, leaveTypeID string, accrual float64) error {
	_, err := tx.Exec(ctx, `
      INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
      VALUES ($1,$2,$3,$4,0,0)
      ON CONFLICT (employee_id, leave_type_id)
      DO UPDATE SET balance = leave_balances.balance + EXCLUDED.balance, updated_at = now()
    `, tenantID, employeeID, leaveTypeID, accrual)
	return err
}

func (s *Store) UpsertBalanceWithCapTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID, leaveTypeID string, accrual, cap float64) error {
	_, err := tx.Exec(ctx, `
      INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
      VALUES ($1,$2,$3,$4,0,0)
      ON CONFLICT (employee_id, leave_type_id)
      DO UPDATE SET balance = LEAST(leave_balances.balance + EXCLUDED.balance, $5), updated_at = now()
    `, tenantID, employeeID, leaveTypeID, accrual, cap)
	return err
}

func (s *Store) RecordAccrualRunTx(ctx context.Context, tx pgx.Tx, tenantID, policyID string, lastAccruedOn time.Time, employeesAccrued int) error {
	_, err := tx.Exec(ctx, `
    INSERT INTO leave_accrual_runs (tenant_id, policy_id, last_accrued_on, employees_accrued)
    VALUES ($1,$2,$3,$4)
    ON CONFLICT (tenant_id, policy_id)
      DO UPDATE SET last_accrued_on = EXCLUDED.last_accrued_on,
                    employees_accrued = EXCLUDED.employees_accrued,
                    updated_at = now()
  `, tenantID, policyID, lastAccruedOn, employeesAccrued)
	return err
}
