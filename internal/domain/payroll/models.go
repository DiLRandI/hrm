package payroll

import "time"

type Schedule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Frequency string    `json:"frequency"`
	PayDay    int       `json:"payDay"`
	CreatedAt time.Time `json:"createdAt"`
}

type Group struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ScheduleID string `json:"scheduleId"`
	Currency   string `json:"currency"`
}

type Element struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	ElementType string  `json:"elementType"`
	CalcType    string  `json:"calcType"`
	Amount      float64 `json:"amount"`
	Taxable     bool    `json:"taxable"`
}

type JournalTemplate struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

type Period struct {
	ID         string    `json:"id"`
	ScheduleID string    `json:"scheduleId"`
	StartDate  time.Time `json:"startDate"`
	EndDate    time.Time `json:"endDate"`
	Status     string    `json:"status"`
}

type Input struct {
	EmployeeID string  `json:"employeeId"`
	ElementID  string  `json:"elementId"`
	Units      float64 `json:"units"`
	Rate       float64 `json:"rate"`
	Amount     float64 `json:"amount"`
	Source     string  `json:"source"`
}

type Adjustment struct {
	ID            string    `json:"id"`
	EmployeeID    string    `json:"employeeId"`
	Description   string    `json:"description"`
	Amount        float64   `json:"amount"`
	EffectiveDate string    `json:"effectiveDate,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Payslip struct {
	ID         string    `json:"id"`
	PeriodID   string    `json:"periodId"`
	EmployeeID string    `json:"employeeId"`
	Gross      float64   `json:"gross"`
	Deductions float64   `json:"deductions"`
	Net        float64   `json:"net"`
	Currency   string    `json:"currency"`
	FileURL    string    `json:"fileUrl"`
	CreatedAt  time.Time `json:"createdAt"`
}

type PayslipKey struct {
	ID         string
	EmployeeID string
}

type RegisterRow struct {
	EmployeeID string
	FirstName  string
	LastName   string
	Gross      float64
	Deductions float64
	Net        float64
	Currency   string
}

type PeriodSummary struct {
	TotalGross      float64        `json:"totalGross"`
	TotalDeductions float64        `json:"totalDeductions"`
	TotalNet        float64        `json:"totalNet"`
	EmployeeCount   int            `json:"employeeCount"`
	Warnings        map[string]int `json:"warnings"`
}

type PeriodDetails struct {
	Status     string
	StartDate  time.Time
	EndDate    time.Time
	ScheduleID string
}

type EmployeePayrollData struct {
	EmployeeID      string
	SalaryPlain     *float64
	SalaryEnc       []byte
	Currency        string
	BankPlain       string
	BankEnc         []byte
	GroupScheduleID string
}

type LeaveWindow struct {
	StartDate time.Time
	EndDate   time.Time
	StartHalf bool
	EndHalf   bool
}
