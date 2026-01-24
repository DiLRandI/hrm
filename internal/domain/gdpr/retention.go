package gdpr

import (
	"context"
	"time"

	"hrm/internal/platform/querier"
)

func ApplyRetention(ctx context.Context, db querier.Querier, tenantID, category string, cutoff time.Time) (int64, error) {
	switch category {
	case DataCategoryAudit:
		tag, err := db.Exec(ctx, `
      DELETE FROM audit_events
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		return tag.RowsAffected(), err
	case DataCategoryLeave:
		var total int64
		tag, err := db.Exec(ctx, `
      DELETE FROM leave_approvals
      WHERE tenant_id = $1 AND decided_at IS NOT NULL AND decided_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = db.Exec(ctx, `
      DELETE FROM leave_requests
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		return total, err
	case DataCategoryPayroll:
		rows, err := db.Query(ctx, `
      SELECT id
      FROM payroll_periods
      WHERE tenant_id = $1 AND end_date < $2
    `, tenantID, cutoff)
		if err != nil {
			return 0, err
		}
		var periodIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return 0, err
			}
			periodIDs = append(periodIDs, id)
		}
		rows.Close()
		if len(periodIDs) == 0 {
			return 0, nil
		}
		var total int64
		queries := []string{
			"DELETE FROM payroll_inputs WHERE tenant_id = $1 AND period_id = ANY($2::uuid[])",
			"DELETE FROM payroll_adjustments WHERE tenant_id = $1 AND period_id = ANY($2::uuid[])",
			"DELETE FROM payroll_results WHERE tenant_id = $1 AND period_id = ANY($2::uuid[])",
			"DELETE FROM payslips WHERE tenant_id = $1 AND period_id = ANY($2::uuid[])",
			"DELETE FROM journal_exports WHERE tenant_id = $1 AND period_id = ANY($2::uuid[])",
			"DELETE FROM payroll_periods WHERE tenant_id = $1 AND id = ANY($2::uuid[])",
		}
		for _, q := range queries {
			tag, err := db.Exec(ctx, q, tenantID, periodIDs)
			total += tag.RowsAffected()
			if err != nil {
				return total, err
			}
		}
		return total, nil
	case DataCategoryPerformance:
		var total int64
		tag, err := db.Exec(ctx, `
      UPDATE feedback
      SET related_goal_id = NULL
      WHERE tenant_id = $1
        AND related_goal_id IN (SELECT id FROM goals WHERE tenant_id = $1 AND updated_at < $2)
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}

		tag, err = db.Exec(ctx, `
      DELETE FROM review_responses
      WHERE tenant_id = $1 AND submitted_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = db.Exec(ctx, `
      DELETE FROM review_tasks
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = db.Exec(ctx, `
      DELETE FROM review_cycles
      WHERE tenant_id = $1 AND end_date < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = db.Exec(ctx, `
      DELETE FROM goal_comments
      WHERE goal_id IN (SELECT id FROM goals WHERE tenant_id = $1 AND updated_at < $2)
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = db.Exec(ctx, `
      DELETE FROM goals
      WHERE tenant_id = $1 AND updated_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = db.Exec(ctx, `
      DELETE FROM feedback
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = db.Exec(ctx, `
      DELETE FROM checkins
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = db.Exec(ctx, `
      DELETE FROM pips
      WHERE tenant_id = $1 AND updated_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		return total, err
	case DataCategoryGDPR:
		var total int64
		tag, err := db.Exec(ctx, `
      DELETE FROM dsar_exports
      WHERE tenant_id = $1 AND completed_at IS NOT NULL AND completed_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		if err != nil {
			return total, err
		}
		tag, err = db.Exec(ctx, `
      DELETE FROM anonymization_jobs
      WHERE tenant_id = $1 AND completed_at IS NOT NULL AND completed_at < $2
    `, tenantID, cutoff)
		total += tag.RowsAffected()
		return total, err
	case DataCategoryAccessLogs:
		tag, err := db.Exec(ctx, `
      DELETE FROM access_logs
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		return tag.RowsAffected(), err
	case DataCategoryNotifications:
		tag, err := db.Exec(ctx, `
      DELETE FROM notifications
      WHERE tenant_id = $1 AND created_at < $2
    `, tenantID, cutoff)
		return tag.RowsAffected(), err
	default:
		return 0, nil
	}
}
