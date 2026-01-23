package performance

import "time"

type Goal struct {
	ID          string    `json:"id"`
	EmployeeID  string    `json:"employeeId"`
	ManagerID   string    `json:"managerId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Metric      string    `json:"metric"`
	DueDate     time.Time `json:"dueDate"`
	Weight      float64   `json:"weight"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
}

type ReviewCycle struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	StartDate  time.Time `json:"startDate"`
	EndDate    time.Time `json:"endDate"`
	Status     string    `json:"status"`
	TemplateID string    `json:"templateId"`
	HRRequired bool      `json:"hrRequired"`
}

type ReviewTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RatingScale any       `json:"ratingScale"`
	Questions   any       `json:"questions"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ReviewTask struct {
	ID         string `json:"id"`
	CycleID    string `json:"cycleId"`
	EmployeeID string `json:"employeeId"`
	ManagerID  string `json:"managerId"`
	Status     string `json:"status"`
	SelfDue    any    `json:"selfDue"`
	ManagerDue any    `json:"managerDue"`
	HRDue      any    `json:"hrDue"`
}

type Feedback struct {
	ID            string    `json:"id"`
	FromUserID    string    `json:"fromUserId"`
	ToEmployeeID  string    `json:"toEmployeeId"`
	Type          string    `json:"type"`
	Message       string    `json:"message"`
	RelatedGoalID any       `json:"relatedGoalId"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Checkin struct {
	ID         string    `json:"id"`
	EmployeeID string    `json:"employeeId"`
	ManagerID  string    `json:"managerId"`
	Notes      string    `json:"notes"`
	Private    bool      `json:"private"`
	CreatedAt  time.Time `json:"createdAt"`
}

type PIP struct {
	ID          string    `json:"id"`
	EmployeeID  string    `json:"employeeId"`
	ManagerID   string    `json:"managerId"`
	HROwnerID   string    `json:"hrOwnerId"`
	Objectives  any       `json:"objectives"`
	Milestones  any       `json:"milestones"`
	ReviewDates any       `json:"reviewDates"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type PerformanceSummary struct {
	GoalsTotal           int            `json:"goalsTotal"`
	GoalsCompleted       int            `json:"goalsCompleted"`
	ReviewTasksTotal     int            `json:"reviewTasksTotal"`
	ReviewTasksCompleted int            `json:"reviewTasksCompleted"`
	ReviewCompletionRate float64        `json:"reviewCompletionRate"`
	RatingDistribution   map[string]int `json:"ratingDistribution"`
}
