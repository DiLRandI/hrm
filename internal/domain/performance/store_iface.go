package performance

import (
	"context"
	"time"
)

type StoreAPI interface {
	EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error)
	EmployeeUserID(ctx context.Context, tenantID, employeeID string) (string, error)
	ManagerIDByEmployeeID(ctx context.Context, tenantID, employeeID string) (string, error)
	IsManagerOfEmployee(ctx context.Context, tenantID, employeeID, managerID string) (bool, error)
	ListGoals(ctx context.Context, tenantID, employeeID, managerID string) ([]Goal, error)
	GetGoal(ctx context.Context, tenantID, goalID string) (GoalDetails, error)
	CreateGoal(ctx context.Context, tenantID, employeeID, managerID, title, description, metric string, dueDate any, weight float64, status string, progress float64) (string, error)
	UpdateGoal(ctx context.Context, tenantID, goalID string, details GoalDetails) error
	CreateGoalComment(ctx context.Context, goalID, authorID, comment string) error
	ListReviewTemplates(ctx context.Context, tenantID string) ([]ReviewTemplate, error)
	CreateReviewTemplate(ctx context.Context, tenantID, name string, ratingJSON, questionsJSON []byte) (string, error)
	ListReviewCycles(ctx context.Context, tenantID string) ([]ReviewCycle, error)
	CreateReviewCycle(ctx context.Context, tenantID, name string, startDate, endDate time.Time, status, templateID string, hrRequired bool) (string, error)
	ListActiveEmployeesForReview(ctx context.Context, tenantID string, employeeIDs []string) ([]EmployeeRef, error)
	CreateReviewTask(ctx context.Context, tenantID, cycleID, employeeID, managerID, status string, selfDue, managerDue, hrDue time.Time) error
	ReviewCycleStatus(ctx context.Context, tenantID, cycleID string) (string, error)
	UpdateReviewCycleStatus(ctx context.Context, tenantID, cycleID, status string) error
	UpdateReviewTasksStatusByCycle(ctx context.Context, tenantID, cycleID, status string) error
	ListReviewTasks(ctx context.Context, tenantID, employeeID, managerID string) ([]ReviewTask, error)
	ReviewTaskContext(ctx context.Context, tenantID, taskID string) (ReviewTaskContext, error)
	ReviewTemplateQuestions(ctx context.Context, tenantID, templateID string) ([]byte, error)
	CreateReviewResponse(ctx context.Context, tenantID, taskID, respondentID, role string, responses []byte, rating any) error
	UpdateReviewTaskStatus(ctx context.Context, tenantID, taskID, status string) error
	ListFeedback(ctx context.Context, tenantID, employeeID, managerID, managerUserID string) ([]Feedback, error)
	CreateFeedback(ctx context.Context, tenantID, fromUserID, toEmployeeID, feedbackType, message string, relatedGoalID any) error
	ListCheckins(ctx context.Context, tenantID, employeeID, managerID string) ([]Checkin, error)
	CreateCheckin(ctx context.Context, tenantID, employeeID, managerID, notes string, private bool) error
	ListPIPs(ctx context.Context, tenantID, employeeID, managerID string) ([]PIP, error)
	CreatePIP(ctx context.Context, tenantID, employeeID, managerID, hrOwnerID string, objectives, milestones, reviewDates []byte, status string) (string, error)
	GetPIP(ctx context.Context, tenantID, pipID string) (string, string, error)
	UpdatePIP(ctx context.Context, tenantID, pipID, status string, objectivesJSON, milestonesJSON, reviewDatesJSON []byte) error
	PerformanceSummaryData(ctx context.Context, tenantID, managerID string) (int, int, int, int, []float64, error)
}
