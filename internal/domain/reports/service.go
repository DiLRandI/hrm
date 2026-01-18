package reports

import "context"

type Service struct {
	Store *Store
}

func NewService(store *Store) *Service {
	return &Service{Store: store}
}

func (s *Service) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	return s.Store.EmployeeIDByUserID(ctx, tenantID, userID)
}

func (s *Service) LeaveBalance(ctx context.Context, tenantID, employeeID string) (float64, error) {
	return s.Store.LeaveBalance(ctx, tenantID, employeeID)
}

func (s *Service) PayslipCount(ctx context.Context, tenantID, employeeID string) (int, error) {
	return s.Store.PayslipCount(ctx, tenantID, employeeID)
}

func (s *Service) GoalCount(ctx context.Context, tenantID, employeeID string) (int, error) {
	return s.Store.GoalCount(ctx, tenantID, employeeID)
}

func (s *Service) PendingApprovals(ctx context.Context, tenantID string) (int, error) {
	return s.Store.PendingApprovals(ctx, tenantID)
}

func (s *Service) TeamGoals(ctx context.Context, tenantID, managerEmployeeID string) (int, error) {
	return s.Store.TeamGoals(ctx, tenantID, managerEmployeeID)
}

func (s *Service) ReviewTasks(ctx context.Context, tenantID, managerEmployeeID string) (int, error) {
	return s.Store.ReviewTasks(ctx, tenantID, managerEmployeeID)
}

func (s *Service) PayrollPeriods(ctx context.Context, tenantID string) (int, error) {
	return s.Store.PayrollPeriods(ctx, tenantID)
}

func (s *Service) LeavePending(ctx context.Context, tenantID string) (int, error) {
	return s.Store.LeavePending(ctx, tenantID)
}

func (s *Service) ReviewCycles(ctx context.Context, tenantID string) (int, error) {
	return s.Store.ReviewCycles(ctx, tenantID)
}

func (s *Service) JobRuns(ctx context.Context, tenantID, jobType string, limit, offset int) ([]map[string]any, error) {
	return s.Store.ListJobRuns(ctx, tenantID, jobType, limit, offset)
}
