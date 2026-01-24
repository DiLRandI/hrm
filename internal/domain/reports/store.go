package reports

import (
	"context"

	"hrm/internal/domain/leave"
	"hrm/internal/platform/querier"
)

type Store struct {
	DB querier.Querier
}

func NewStore(db querier.Querier) *Store {
	return &Store{DB: db}
}

func (s *Store) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	var employeeID string
	err := s.DB.QueryRow(ctx, "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", tenantID, userID).Scan(&employeeID)
	if err != nil {
		return "", err
	}
	return employeeID, nil
}

func (s *Store) LeaveBalance(ctx context.Context, tenantID, employeeID string) (float64, error) {
	var leaveBalance float64
	if err := s.DB.QueryRow(ctx, "SELECT COALESCE(SUM(balance),0) FROM leave_balances WHERE tenant_id = $1 AND employee_id = $2", tenantID, employeeID).Scan(&leaveBalance); err != nil {
		return 0, err
	}
	return leaveBalance, nil
}

func (s *Store) PayslipCount(ctx context.Context, tenantID, employeeID string) (int, error) {
	var payslipCount int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM payslips WHERE tenant_id = $1 AND employee_id = $2", tenantID, employeeID).Scan(&payslipCount); err != nil {
		return 0, err
	}
	return payslipCount, nil
}

func (s *Store) GoalCount(ctx context.Context, tenantID, employeeID string) (int, error) {
	var goalCount int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM goals WHERE tenant_id = $1 AND employee_id = $2", tenantID, employeeID).Scan(&goalCount); err != nil {
		return 0, err
	}
	return goalCount, nil
}

func (s *Store) PendingApprovals(ctx context.Context, tenantID string) (int, error) {
	var pendingApprovals int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status IN ($2,$3)", tenantID, leave.StatusPending, leave.StatusPendingHR).Scan(&pendingApprovals); err != nil {
		return 0, err
	}
	return pendingApprovals, nil
}

func (s *Store) TeamGoals(ctx context.Context, tenantID, managerEmployeeID string) (int, error) {
	var teamGoals int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM goals WHERE tenant_id = $1 AND manager_id = $2", tenantID, managerEmployeeID).Scan(&teamGoals); err != nil {
		return 0, err
	}
	return teamGoals, nil
}

func (s *Store) ReviewTasks(ctx context.Context, tenantID, managerEmployeeID string) (int, error) {
	var reviewTasks int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM review_tasks WHERE tenant_id = $1 AND manager_id = $2", tenantID, managerEmployeeID).Scan(&reviewTasks); err != nil {
		return 0, err
	}
	return reviewTasks, nil
}

func (s *Store) PayrollPeriods(ctx context.Context, tenantID string) (int, error) {
	var payrollPeriods int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM payroll_periods WHERE tenant_id = $1", tenantID).Scan(&payrollPeriods); err != nil {
		return 0, err
	}
	return payrollPeriods, nil
}

func (s *Store) LeavePending(ctx context.Context, tenantID string) (int, error) {
	var leavePending int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM leave_requests WHERE tenant_id = $1 AND status IN ($2,$3)", tenantID, leave.StatusPending, leave.StatusPendingHR).Scan(&leavePending); err != nil {
		return 0, err
	}
	return leavePending, nil
}

func (s *Store) ReviewCycles(ctx context.Context, tenantID string) (int, error) {
	var reviewCycles int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM review_cycles WHERE tenant_id = $1", tenantID).Scan(&reviewCycles); err != nil {
		return 0, err
	}
	return reviewCycles, nil
}

func (s *Store) ListJobRuns(ctx context.Context, tenantID, jobType string, limit, offset int) ([]map[string]any, error) {
	query := `
    SELECT id, job_type, status, details_json, started_at, completed_at
    FROM job_runs
    WHERE tenant_id = $1
  `
	args := []any{tenantID}
	if jobType != "" {
		query += " AND job_type = $2"
		args = append(args, jobType)
	}
	query += " ORDER BY started_at DESC LIMIT $3 OFFSET $4"
	args = append(args, limit, offset)

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []map[string]any
	for rows.Next() {
		var id, jobTypeVal, status string
		var details any
		var startedAt, completedAt any
		if err := rows.Scan(&id, &jobTypeVal, &status, &details, &startedAt, &completedAt); err != nil {
			return nil, err
		}
		runs = append(runs, map[string]any{
			"id":          id,
			"jobType":     jobTypeVal,
			"status":      status,
			"details":     details,
			"startedAt":   startedAt,
			"completedAt": completedAt,
		})
	}
	return runs, nil
}
