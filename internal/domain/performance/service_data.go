package performance

import (
	"context"
	"fmt"
	"time"
)

func (s *Service) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	return s.store.EmployeeIDByUserID(ctx, tenantID, userID)
}

func (s *Service) EmployeeUserID(ctx context.Context, tenantID, employeeID string) (string, error) {
	return s.store.EmployeeUserID(ctx, tenantID, employeeID)
}

func (s *Service) ManagerIDByEmployeeID(ctx context.Context, tenantID, employeeID string) (string, error) {
	return s.store.ManagerIDByEmployeeID(ctx, tenantID, employeeID)
}

func (s *Service) IsManagerOfEmployee(ctx context.Context, tenantID, employeeID, managerID string) (bool, error) {
	return s.store.IsManagerOfEmployee(ctx, tenantID, employeeID, managerID)
}

func (s *Service) ListGoals(ctx context.Context, tenantID, employeeID, managerID string) ([]Goal, error) {
	return s.store.ListGoals(ctx, tenantID, employeeID, managerID)
}

func (s *Service) GetGoal(ctx context.Context, tenantID, goalID string) (GoalDetails, error) {
	return s.store.GetGoal(ctx, tenantID, goalID)
}

func (s *Service) CreateGoal(ctx context.Context, tenantID, employeeID, managerID, title, description, metric string, dueDate any, weight float64, status string, progress float64) (string, error) {
	return s.store.CreateGoal(ctx, tenantID, employeeID, managerID, title, description, metric, dueDate, weight, status, progress)
}

func (s *Service) UpdateGoal(ctx context.Context, tenantID, goalID string, details GoalDetails) error {
	return s.store.UpdateGoal(ctx, tenantID, goalID, details)
}

func (s *Service) CreateGoalComment(ctx context.Context, goalID, authorID, comment string) error {
	return s.store.CreateGoalComment(ctx, goalID, authorID, comment)
}

func (s *Service) ListReviewTemplates(ctx context.Context, tenantID string) ([]ReviewTemplate, error) {
	return s.store.ListReviewTemplates(ctx, tenantID)
}

func (s *Service) CreateReviewTemplate(ctx context.Context, tenantID, name string, ratingJSON, questionsJSON []byte) (string, error) {
	return s.store.CreateReviewTemplate(ctx, tenantID, name, ratingJSON, questionsJSON)
}

func (s *Service) ListReviewCycles(ctx context.Context, tenantID string) ([]ReviewCycle, error) {
	return s.store.ListReviewCycles(ctx, tenantID)
}

func (s *Service) CreateReviewCycle(ctx context.Context, tenantID, name string, startDate, endDate time.Time, status, templateID string, hrRequired bool) (string, error) {
	return s.store.CreateReviewCycle(ctx, tenantID, name, startDate, endDate, status, templateID, hrRequired)
}

func (s *Service) ListActiveEmployeesForReview(ctx context.Context, tenantID string, employeeIDs []string) ([]EmployeeRef, error) {
	return s.store.ListActiveEmployeesForReview(ctx, tenantID, employeeIDs)
}

func (s *Service) CreateReviewTask(ctx context.Context, tenantID, cycleID, employeeID, managerID, status string, selfDue, managerDue, hrDue time.Time) error {
	return s.store.CreateReviewTask(ctx, tenantID, cycleID, employeeID, managerID, status, selfDue, managerDue, hrDue)
}

func (s *Service) ReviewCycleStatus(ctx context.Context, tenantID, cycleID string) (string, error) {
	return s.store.ReviewCycleStatus(ctx, tenantID, cycleID)
}

func (s *Service) UpdateReviewCycleStatus(ctx context.Context, tenantID, cycleID, status string) error {
	return s.store.UpdateReviewCycleStatus(ctx, tenantID, cycleID, status)
}

func (s *Service) UpdateReviewTasksStatusByCycle(ctx context.Context, tenantID, cycleID, status string) error {
	return s.store.UpdateReviewTasksStatusByCycle(ctx, tenantID, cycleID, status)
}

func (s *Service) ListReviewTasks(ctx context.Context, tenantID, employeeID, managerID string) ([]ReviewTask, error) {
	return s.store.ListReviewTasks(ctx, tenantID, employeeID, managerID)
}

func (s *Service) ReviewTaskContext(ctx context.Context, tenantID, taskID string) (ReviewTaskContext, error) {
	return s.store.ReviewTaskContext(ctx, tenantID, taskID)
}

func (s *Service) ReviewTemplateQuestions(ctx context.Context, tenantID, templateID string) ([]byte, error) {
	return s.store.ReviewTemplateQuestions(ctx, tenantID, templateID)
}

func (s *Service) CreateReviewResponse(ctx context.Context, tenantID, taskID, respondentID, role string, responses []byte, rating any) error {
	return s.store.CreateReviewResponse(ctx, tenantID, taskID, respondentID, role, responses, rating)
}

func (s *Service) UpdateReviewTaskStatus(ctx context.Context, tenantID, taskID, status string) error {
	return s.store.UpdateReviewTaskStatus(ctx, tenantID, taskID, status)
}

func (s *Service) ListFeedback(ctx context.Context, tenantID, employeeID, managerID, managerUserID string) ([]Feedback, error) {
	return s.store.ListFeedback(ctx, tenantID, employeeID, managerID, managerUserID)
}

func (s *Service) CreateFeedback(ctx context.Context, tenantID, fromUserID, toEmployeeID, feedbackType, message string, relatedGoalID any) error {
	return s.store.CreateFeedback(ctx, tenantID, fromUserID, toEmployeeID, feedbackType, message, relatedGoalID)
}

func (s *Service) ListCheckins(ctx context.Context, tenantID, employeeID, managerID string) ([]Checkin, error) {
	return s.store.ListCheckins(ctx, tenantID, employeeID, managerID)
}

func (s *Service) CreateCheckin(ctx context.Context, tenantID, employeeID, managerID, notes string, private bool) error {
	return s.store.CreateCheckin(ctx, tenantID, employeeID, managerID, notes, private)
}

func (s *Service) ListPIPs(ctx context.Context, tenantID, employeeID, managerID string) ([]PIP, error) {
	return s.store.ListPIPs(ctx, tenantID, employeeID, managerID)
}

func (s *Service) CreatePIP(ctx context.Context, tenantID, employeeID, managerID, hrOwnerID string, objectives, milestones, reviewDates []byte, status string) (string, error) {
	return s.store.CreatePIP(ctx, tenantID, employeeID, managerID, hrOwnerID, objectives, milestones, reviewDates, status)
}

func (s *Service) GetPIP(ctx context.Context, tenantID, pipID string) (string, string, error) {
	return s.store.GetPIP(ctx, tenantID, pipID)
}

func (s *Service) UpdatePIP(ctx context.Context, tenantID, pipID, status string, objectivesJSON, milestonesJSON, reviewDatesJSON []byte) error {
	return s.store.UpdatePIP(ctx, tenantID, pipID, status, objectivesJSON, milestonesJSON, reviewDatesJSON)
}

func (s *Service) PerformanceSummary(ctx context.Context, tenantID, managerID string) (PerformanceSummary, error) {
	goalsTotal, goalsCompleted, tasksTotal, tasksCompleted, ratings, err := s.store.PerformanceSummaryData(ctx, tenantID, managerID)
	if err != nil {
		return PerformanceSummary{}, err
	}
	return buildPerformanceSummary(goalsTotal, goalsCompleted, tasksTotal, tasksCompleted, ratings), nil
}

func buildPerformanceSummary(goalsTotal, goalsCompleted, tasksTotal, tasksCompleted int, ratings []float64) PerformanceSummary {
	summary := PerformanceSummary{
		GoalsTotal:           goalsTotal,
		GoalsCompleted:       goalsCompleted,
		ReviewTasksTotal:     tasksTotal,
		ReviewTasksCompleted: tasksCompleted,
		RatingDistribution:   map[string]int{},
	}
	for _, rating := range ratings {
		key := fmt.Sprintf("%d", int(rating+0.5))
		summary.RatingDistribution[key]++
	}
	if tasksTotal > 0 {
		summary.ReviewCompletionRate = float64(tasksCompleted) / float64(tasksTotal)
	}
	return summary
}
