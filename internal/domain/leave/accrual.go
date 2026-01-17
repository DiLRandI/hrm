package leave

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccrualSummary struct {
	PoliciesProcessed int
	EmployeesAccrued  int
}

type policyRow struct {
	ID            string
	LeaveTypeID   string
	AccrualRate   float64
	AccrualPeriod string
	Entitlement   float64
	CarryOver     float64
}

func ApplyAccruals(ctx context.Context, db *pgxpool.Pool, tenantID string, now time.Time) (AccrualSummary, error) {
	var summary AccrualSummary

	rows, err := db.Query(ctx, `
    SELECT id, leave_type_id, accrual_rate, accrual_period, entitlement, carry_over_limit
    FROM leave_policies
    WHERE tenant_id = $1 AND accrual_rate IS NOT NULL
  `, tenantID)
	if err != nil {
		return summary, err
	}
	defer rows.Close()

	policies := make([]policyRow, 0)
	for rows.Next() {
		var p policyRow
		if err := rows.Scan(&p.ID, &p.LeaveTypeID, &p.AccrualRate, &p.AccrualPeriod, &p.Entitlement, &p.CarryOver); err != nil {
			return summary, err
		}
		policies = append(policies, p)
	}

	for _, policy := range policies {
		periodStart := accrualPeriodStart(now, policy.AccrualPeriod)
		if periodStart.IsZero() {
			continue
		}

		var last time.Time
		err := db.QueryRow(ctx, `
      SELECT last_accrued_on
      FROM leave_accrual_runs
      WHERE tenant_id = $1 AND policy_id = $2
    `, tenantID, policy.ID).Scan(&last)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return summary, err
		}
		if !last.IsZero() && !last.Before(periodStart) {
			continue
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return summary, err
		}

		employees, err := tx.Query(ctx, `
      SELECT id, start_date
      FROM employees
      WHERE tenant_id = $1 AND status = 'active'
    `, tenantID)
		if err != nil {
			_ = tx.Rollback(ctx)
			return summary, err
		}

		for employees.Next() {
			var employeeID string
			var startDate *time.Time
			if err := employees.Scan(&employeeID, &startDate); err != nil {
				employees.Close()
				_ = tx.Rollback(ctx)
				return summary, err
			}
			accrual := policy.AccrualRate
			if startDate != nil && startDate.After(periodStart) {
				accrual = proratedAccrual(policy.AccrualRate, *startDate, periodStart, now, policy.AccrualPeriod)
			}
			if accrual <= 0 {
				continue
			}

			var capValue *float64
			if policy.Entitlement > 0 {
				cap := policy.Entitlement + policy.CarryOver
				capValue = &cap
			}

			if capValue == nil {
				_, err = tx.Exec(ctx, `
          INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
          VALUES ($1,$2,$3,$4,0,0)
          ON CONFLICT (employee_id, leave_type_id)
          DO UPDATE SET balance = leave_balances.balance + EXCLUDED.balance, updated_at = now()
        `, tenantID, employeeID, policy.LeaveTypeID, accrual)
			} else {
				_, err = tx.Exec(ctx, `
          INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
          VALUES ($1,$2,$3,$4,0,0)
          ON CONFLICT (employee_id, leave_type_id)
          DO UPDATE SET balance = LEAST(leave_balances.balance + EXCLUDED.balance, $5), updated_at = now()
        `, tenantID, employeeID, policy.LeaveTypeID, accrual, *capValue)
			}
			if err != nil {
				employees.Close()
				_ = tx.Rollback(ctx)
				return summary, err
			}
			summary.EmployeesAccrued++
		}
		employees.Close()

		_, err = tx.Exec(ctx, `
      INSERT INTO leave_accrual_runs (tenant_id, policy_id, last_accrued_on)
      VALUES ($1,$2,$3)
      ON CONFLICT (tenant_id, policy_id) DO UPDATE SET last_accrued_on = EXCLUDED.last_accrued_on
    `, tenantID, policy.ID, periodStart)
		if err != nil {
			_ = tx.Rollback(ctx)
			return summary, err
		}
		if err := tx.Commit(ctx); err != nil {
			return summary, err
		}
		summary.PoliciesProcessed++
	}

	return summary, nil
}

func accrualPeriodStart(now time.Time, period string) time.Time {
	switch period {
	case "weekly":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	case "monthly":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "yearly":
		return time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location())
	default:
		return time.Time{}
	}
}

func proratedAccrual(rate float64, start, periodStart, now time.Time, period string) float64 {
	switch period {
	case "weekly":
		end := periodStart.AddDate(0, 0, 7)
		total := end.Sub(periodStart).Hours()
		remaining := end.Sub(start).Hours()
		if remaining < 0 {
			return 0
		}
		return rate * (remaining / total)
	case "monthly":
		end := periodStart.AddDate(0, 1, 0)
		total := end.Sub(periodStart).Hours()
		remaining := end.Sub(start).Hours()
		if remaining < 0 {
			return 0
		}
		return rate * (remaining / total)
	case "yearly":
		end := periodStart.AddDate(1, 0, 0)
		total := end.Sub(periodStart).Hours()
		remaining := end.Sub(start).Hours()
		if remaining < 0 {
			return 0
		}
		return rate * (remaining / total)
	default:
		return rate
	}
}
