package core

import "time"

type Employee struct {
	ID             string     `json:"id"`
	UserID         string     `json:"userId"`
	EmployeeNumber string     `json:"employeeNumber"`
	FirstName      string     `json:"firstName"`
	LastName       string     `json:"lastName"`
	Email          string     `json:"email"`
	Phone          string     `json:"phone"`
	DateOfBirth    *time.Time `json:"dateOfBirth,omitempty"`
	Address        string     `json:"address"`
	NationalID     string     `json:"nationalId,omitempty"`
	BankAccount    string     `json:"bankAccount,omitempty"`
	Salary         *float64   `json:"salary,omitempty"`
	Currency       string     `json:"currency"`
	EmploymentType string     `json:"employmentType"`
	DepartmentID   string     `json:"departmentId"`
	ManagerID      string     `json:"managerId"`
	PayGroupID     string     `json:"payGroupId"`
	StartDate      *time.Time `json:"startDate,omitempty"`
	EndDate        *time.Time `json:"endDate,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type Department struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ParentID  string    `json:"parentId"`
	ManagerID string    `json:"managerId"`
	CreatedAt time.Time `json:"createdAt"`
}
