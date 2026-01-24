package leave

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type StoreAPI interface {
	ListTypes(ctx context.Context, tenantID string) ([]LeaveType, error)
	CreateType(ctx context.Context, tenantID string, payload LeaveType) (string, error)
	ListPolicies(ctx context.Context, tenantID string) ([]LeavePolicy, error)
	CreatePolicy(ctx context.Context, tenantID string, payload LeavePolicy) (string, error)
	ListHolidays(ctx context.Context, tenantID string) ([]map[string]any, error)
	CreateHoliday(ctx context.Context, tenantID string, date time.Time, name, region string) (string, error)
	DeleteHoliday(ctx context.Context, tenantID, holidayID string) error
	ListBalances(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error)
	AdjustBalance(ctx context.Context, tenantID, employeeID, leaveTypeID, reason, userID string, amount float64) error
	ListRequests(ctx context.Context, tenantID, roleName, employeeID, managerEmployeeID string, limit, offset int) (RequestListResult, error)
	RequiresHRApproval(ctx context.Context, tenantID, leaveTypeID string) (bool, error)
	CreateRequest(ctx context.Context, tenantID, employeeID, leaveTypeID, reason string, startDate, endDate time.Time, days float64, status string) (string, error)
	AddPendingBalance(ctx context.Context, tenantID, employeeID, leaveTypeID string, days float64) error
	ManagerUserIDForEmployee(ctx context.Context, tenantID, employeeID string) (string, error)
	InsertApproval(ctx context.Context, tenantID, requestID, approverID, status string) error
	UpdateRequestStatus(ctx context.Context, requestID, status, approverID string) error
	UpdateRequestStatusSimple(ctx context.Context, requestID, status string) error
	HRUserIDs(ctx context.Context, tenantID string) ([]string, error)
	RequestInfo(ctx context.Context, tenantID, requestID string) (string, string, float64, string, error)
	UpdateBalanceOnApproval(ctx context.Context, tenantID, employeeID, leaveTypeID string, days float64) error
	UpdateBalanceOnReject(ctx context.Context, tenantID, employeeID, leaveTypeID string, days float64) error
	EmployeeUserAndLeaveType(ctx context.Context, tenantID, leaveTypeID, employeeID string) (string, string, error)
	CalendarEntries(ctx context.Context, tenantID string, statuses []string, employeeID string) ([]CalendarEntry, error)
	CalendarExportRows(ctx context.Context, tenantID string, statuses []string, employeeID string, managerID string) ([]CalendarExportRow, error)
	ReportBalances(ctx context.Context, tenantID string) ([]map[string]any, error)
	ReportUsage(ctx context.Context, tenantID string) ([]map[string]any, error)
}

type AccrualStore interface {
	ListAccrualPolicies(ctx context.Context, tenantID string) ([]policyRow, error)
	LastAccruedOn(ctx context.Context, tenantID, policyID string) (time.Time, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
	ListActiveEmployeesTx(ctx context.Context, tx pgx.Tx, tenantID string) (map[string]*time.Time, error)
	UpsertBalanceTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID, leaveTypeID string, accrual float64) error
	UpsertBalanceWithCapTx(ctx context.Context, tx pgx.Tx, tenantID, employeeID, leaveTypeID string, accrual, cap float64) error
	RecordAccrualRunTx(ctx context.Context, tx pgx.Tx, tenantID, policyID string, lastAccruedOn time.Time, employeesAccrued int) error
}
