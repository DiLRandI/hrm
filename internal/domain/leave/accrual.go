package leave

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
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

func ApplyAccruals(ctx context.Context, store AccrualStore, tenantID string, now time.Time) (AccrualSummary, error) {
	var summary AccrualSummary

	policies, err := store.ListAccrualPolicies(ctx, tenantID)
	if err != nil {
		return summary, err
	}

	for _, policy := range policies {
		periodStart := accrualPeriodStart(now, policy.AccrualPeriod)
		if periodStart.IsZero() {
			continue
		}

		last, err := store.LastAccruedOn(ctx, tenantID, policy.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return summary, err
		}
		if !last.IsZero() && !last.Before(periodStart) {
			continue
		}

		tx, err := store.BeginTx(ctx)
		if err != nil {
			return summary, err
		}

		employees, err := store.ListActiveEmployeesTx(ctx, tx, tenantID)
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				slog.Warn("leave accrual rollback failed", "err", rbErr)
			}
			return summary, err
		}

		for employeeID, startDate := range employees {
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
				err = store.UpsertBalanceTx(ctx, tx, tenantID, employeeID, policy.LeaveTypeID, accrual)
			} else {
				err = store.UpsertBalanceWithCapTx(ctx, tx, tenantID, employeeID, policy.LeaveTypeID, accrual, *capValue)
			}
			if err != nil {
				if rbErr := tx.Rollback(ctx); rbErr != nil {
					slog.Warn("leave accrual rollback failed", "err", rbErr)
				}
				return summary, err
			}
			summary.EmployeesAccrued++
		}

		if err := store.RecordAccrualRunTx(ctx, tx, tenantID, policy.ID, periodStart, summary.EmployeesAccrued); err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				slog.Warn("leave accrual rollback failed", "err", rbErr)
			}
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
