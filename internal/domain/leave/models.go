package leave

import "time"

type LeaveType struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	IsPaid      bool      `json:"isPaid"`
	RequiresDoc bool      `json:"requiresDoc"`
	CreatedAt   time.Time `json:"createdAt"`
}

type LeavePolicy struct {
	ID                 string  `json:"id"`
	LeaveTypeID        string  `json:"leaveTypeId"`
	AccrualRate        float64 `json:"accrualRate"`
	AccrualPeriod      string  `json:"accrualPeriod"`
	Entitlement        float64 `json:"entitlement"`
	CarryOver          float64 `json:"carryOverLimit"`
	AllowNegative      bool    `json:"allowNegative"`
	RequiresHRApproval bool    `json:"requiresHrApproval"`
}

type LeaveRequest struct {
	ID          string    `json:"id"`
	EmployeeID  string    `json:"employeeId"`
	LeaveTypeID string    `json:"leaveTypeId"`
	StartDate   time.Time `json:"startDate"`
	EndDate     time.Time `json:"endDate"`
	Days        float64   `json:"days"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}
