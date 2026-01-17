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
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
	Status    string    `json:"status"`
}
