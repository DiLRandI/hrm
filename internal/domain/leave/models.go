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
	ID          string                 `json:"id"`
	EmployeeID  string                 `json:"employeeId"`
	LeaveTypeID string                 `json:"leaveTypeId"`
	StartDate   time.Time              `json:"startDate"`
	EndDate     time.Time              `json:"endDate"`
	StartHalf   bool                   `json:"startHalf"`
	EndHalf     bool                   `json:"endHalf"`
	Days        float64                `json:"days"`
	Reason      string                 `json:"reason"`
	Status      string                 `json:"status"`
	Documents   []LeaveRequestDocument `json:"documents,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
}

type LeaveRequestDocument struct {
	ID             string    `json:"id"`
	LeaveRequestID string    `json:"leaveRequestId,omitempty"`
	FileName       string    `json:"fileName"`
	ContentType    string    `json:"contentType"`
	FileSize       int64     `json:"fileSize"`
	UploadedBy     string    `json:"uploadedBy,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type LeaveRequestDocumentUpload struct {
	FileName    string
	ContentType string
	FileSize    int64
	Data        []byte
}
