package gdpr

import (
	"context"
	"encoding/json"
)

func (s *Store) DSARLeaveRequests(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(lr) FROM leave_requests lr WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARPayrollResults(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(pr) FROM payroll_results pr WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARPayslips(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(ps) FROM payslips ps WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARGoals(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(g) FROM goals g WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARGoalComments(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(gc) FROM goal_comments gc JOIN goals g ON gc.goal_id = g.id WHERE g.tenant_id = $1 AND g.employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARFeedback(ctx context.Context, tenantID, employeeID, userID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(f) FROM feedback f WHERE tenant_id = $1 AND (to_employee_id = $2 OR from_user_id = $3)`, tenantID, employeeID, userID)
}

func (s *Store) DSARCheckins(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(c) FROM checkins c WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARPIPs(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(p) FROM pips p WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARReviewTasks(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(rt) FROM review_tasks rt WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARReviewResponses(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(rr) FROM review_responses rr JOIN review_tasks rt ON rr.task_id = rt.id WHERE rr.tenant_id = $1 AND rt.employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARConsentRecords(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(cr) FROM consent_records cr WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARNotifications(ctx context.Context, tenantID, userID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(n) FROM notifications n WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID)
}

func (s *Store) DSARAccessLogs(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(al) FROM access_logs al WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID)
}

func (s *Store) DSARManagerHistory(ctx context.Context, employeeID string) ([]map[string]any, error) {
	return s.queryRowsAsJSON(ctx, `SELECT row_to_json(mr) FROM manager_relations mr WHERE mr.employee_id = $1`, employeeID)
}

func (s *Store) queryRowsAsJSON(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var rowJSON []byte
		if err := rows.Scan(&rowJSON); err != nil {
			return nil, err
		}
		var row map[string]any
		if err := json.Unmarshal(rowJSON, &row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
