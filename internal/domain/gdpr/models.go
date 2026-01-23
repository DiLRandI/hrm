package gdpr

import "time"

type RetentionPolicy struct {
	ID            string `json:"id"`
	DataCategory  string `json:"dataCategory"`
	RetentionDays int    `json:"retentionDays"`
}

type RetentionRun struct {
	ID           string    `json:"id"`
	DataCategory string    `json:"dataCategory"`
	CutoffDate   time.Time `json:"cutoffDate"`
	Status       string    `json:"status"`
	DeletedCount int64     `json:"deletedCount"`
	StartedAt    time.Time `json:"startedAt"`
	CompletedAt  time.Time `json:"completedAt"`
}

type RetentionRunSummary struct {
	DataCategory string    `json:"dataCategory"`
	CutoffDate   time.Time `json:"cutoffDate"`
	Status       string    `json:"status"`
	DeletedCount int64     `json:"deletedCount"`
}

type ConsentRecord struct {
	ID          string `json:"id"`
	EmployeeID  string `json:"employeeId"`
	ConsentType string `json:"consentType"`
	GrantedAt   any    `json:"grantedAt"`
	RevokedAt   any    `json:"revokedAt"`
}

type DSARExport struct {
	ID            string    `json:"id"`
	EmployeeID    string    `json:"employeeId"`
	RequestedBy   string    `json:"requestedBy"`
	Status        string    `json:"status"`
	FilePath      string    `json:"filePath"`
	DownloadToken string    `json:"downloadToken"`
	RequestedAt   time.Time `json:"requestedAt"`
	CompletedAt   any       `json:"completedAt"`
}

type AnonymizationJob struct {
	ID            string    `json:"id"`
	EmployeeID    string    `json:"employeeId"`
	Status        string    `json:"status"`
	Reason        string    `json:"reason"`
	RequestedAt   time.Time `json:"requestedAt"`
	CompletedAt   any       `json:"completedAt"`
	FilePath      string    `json:"filePath,omitempty"`
	DownloadToken string    `json:"downloadToken,omitempty"`
}

type AccessLog struct {
	ID         string    `json:"id"`
	ActorID    string    `json:"actorUserId"`
	EmployeeID string    `json:"employeeId"`
	Fields     []string  `json:"fields"`
	RequestID  string    `json:"requestId"`
	CreatedAt  time.Time `json:"createdAt"`
}
