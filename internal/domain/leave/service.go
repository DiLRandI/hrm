package leave

import (
	"context"
	"errors"
	"time"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
)

type Service struct {
	Store     StoreAPI
	Employees EmployeeLookup
}

type EmployeeLookup interface {
	EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error)
	ManagerIDByEmployeeID(ctx context.Context, tenantID, employeeID string) (string, error)
	IsManagerOf(ctx context.Context, tenantID, managerEmployeeID, employeeID string) (bool, error)
}

var (
	ErrHRApprovalRequired = errors.New("hr approval required")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidState       = errors.New("invalid state")
)

func NewService(store StoreAPI, coreStore *core.Store) *Service {
	return &Service{Store: store, Employees: coreStore}
}

func (s *Service) ListTypes(ctx context.Context, tenantID string) ([]LeaveType, error) {
	return s.Store.ListTypes(ctx, tenantID)
}

func (s *Service) CreateType(ctx context.Context, tenantID string, payload LeaveType) (string, error) {
	return s.Store.CreateType(ctx, tenantID, payload)
}

func (s *Service) LeaveTypeRequiresDoc(ctx context.Context, tenantID, leaveTypeID string) (bool, error) {
	return s.Store.LeaveTypeRequiresDoc(ctx, tenantID, leaveTypeID)
}

func (s *Service) ListPolicies(ctx context.Context, tenantID string) ([]LeavePolicy, error) {
	return s.Store.ListPolicies(ctx, tenantID)
}

func (s *Service) CreatePolicy(ctx context.Context, tenantID string, payload LeavePolicy) (string, error) {
	return s.Store.CreatePolicy(ctx, tenantID, payload)
}

func (s *Service) ListHolidays(ctx context.Context, tenantID string) ([]map[string]any, error) {
	return s.Store.ListHolidays(ctx, tenantID)
}

func (s *Service) CreateHoliday(ctx context.Context, tenantID string, date time.Time, name, region string) (string, error) {
	return s.Store.CreateHoliday(ctx, tenantID, date, name, region)
}

func (s *Service) DeleteHoliday(ctx context.Context, tenantID, holidayID string) error {
	return s.Store.DeleteHoliday(ctx, tenantID, holidayID)
}

func (s *Service) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	if s.Employees == nil {
		return "", nil
	}
	return s.Employees.EmployeeIDByUserID(ctx, tenantID, userID)
}

func (s *Service) IsManagerOf(ctx context.Context, tenantID, managerEmployeeID, employeeID string) (bool, error) {
	if s.Employees == nil {
		return false, nil
	}
	return s.Employees.IsManagerOf(ctx, tenantID, managerEmployeeID, employeeID)
}

func (s *Service) ListBalances(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.Store.ListBalances(ctx, tenantID, employeeID)
}

func (s *Service) AdjustBalance(ctx context.Context, tenantID, employeeID, leaveTypeID, reason, userID string, amount float64) error {
	return s.Store.AdjustBalance(ctx, tenantID, employeeID, leaveTypeID, reason, userID, amount)
}

func (s *Service) RunAccruals(ctx context.Context, tenantID string, now time.Time) (AccrualSummary, error) {
	accrualStore, ok := s.Store.(AccrualStore)
	if !ok {
		return AccrualSummary{}, errors.New("accrual store not configured")
	}
	return ApplyAccruals(ctx, accrualStore, tenantID, now)
}

type RequestListResult struct {
	Requests []LeaveRequest
	Total    int
}

func (s *Service) ListRequests(ctx context.Context, tenantID, roleName, employeeID, managerEmployeeID string, limit, offset int) (RequestListResult, error) {
	return s.Store.ListRequests(ctx, tenantID, roleName, employeeID, managerEmployeeID, limit, offset)
}

func (s *Service) GetRequest(ctx context.Context, tenantID, requestID string) (LeaveRequest, error) {
	return s.Store.GetRequest(ctx, tenantID, requestID)
}

type CreateRequestResult struct {
	ID            string
	Status        string
	ManagerUserID string
	HRUserIDs     []string
}

func (s *Service) CreateRequest(ctx context.Context, tenantID, employeeID, leaveTypeID, reason string, startDate, endDate time.Time, startHalf, endHalf bool, days float64) (CreateRequestResult, error) {
	result := CreateRequestResult{Status: StatusPending}
	requiresHR, err := s.Store.RequiresHRApproval(ctx, tenantID, leaveTypeID)
	if err != nil {
		requiresHR = false
	}

	if id, err := s.Store.CreateRequest(ctx, tenantID, employeeID, leaveTypeID, reason, startDate, endDate, startHalf, endHalf, days, StatusPending); err != nil {
		return result, err
	} else {
		result.ID = id
	}

	if err := s.Store.AddPendingBalance(ctx, tenantID, employeeID, leaveTypeID, days); err != nil {
		return result, err
	}

	if managerUserID, err := s.Store.ManagerUserIDForEmployee(ctx, tenantID, employeeID); err != nil {
		result.ManagerUserID = ""
	} else {
		result.ManagerUserID = managerUserID
	}
	if result.ManagerUserID == "" {
		requiresHR = true
	}

	if result.ManagerUserID != "" {
		if err := s.Store.InsertApproval(ctx, tenantID, result.ID, result.ManagerUserID, StatusPending); err != nil {
			return result, err
		}
	} else if requiresHR {
		if err := s.Store.UpdateRequestStatusSimple(ctx, result.ID, StatusPendingHR); err != nil {
			return result, err
		}
		if hrUserIDs, err := s.Store.HRUserIDs(ctx, tenantID); err == nil {
			result.HRUserIDs = hrUserIDs
		}
	}

	if result.ManagerUserID == "" && requiresHR {
		result.Status = StatusPendingHR
	}

	return result, nil
}

func (s *Service) CreateRequestDocument(ctx context.Context, tenantID, requestID string, payload LeaveRequestDocumentUpload, uploadedBy string) (LeaveRequestDocument, error) {
	return s.Store.CreateRequestDocument(ctx, tenantID, requestID, payload, uploadedBy)
}

func (s *Service) RequestDocumentData(ctx context.Context, tenantID, requestID, documentID string) (LeaveRequestDocument, []byte, error) {
	return s.Store.RequestDocumentData(ctx, tenantID, requestID, documentID)
}

type ApprovalResult struct {
	EmployeeID    string
	LeaveTypeID   string
	LeaveTypeName string
	Status        string
	EmployeeUser  string
	ManagerUserID string
	HRUserIDs     []string
	FinalApproval bool
}

func (s *Service) ApproveRequest(ctx context.Context, tenantID, requestID, approverUserID, roleName string) (ApprovalResult, error) {
	result := ApprovalResult{Status: StatusApproved}
	employeeID, leaveTypeID, days, currentStatus, err := s.Store.RequestInfo(ctx, tenantID, requestID)
	if err != nil {
		return result, err
	}
	result.EmployeeID = employeeID
	result.LeaveTypeID = leaveTypeID

	requiresHR, err := s.Store.RequiresHRApproval(ctx, tenantID, leaveTypeID)
	if err != nil {
		requiresHR = false
	}

	if currentStatus == StatusPendingHR && roleName != auth.RoleHR {
		return result, ErrHRApprovalRequired
	}

	if roleName == auth.RoleManager {
		if s.Employees != nil {
			managerEmployeeID, err := s.Employees.ManagerIDByEmployeeID(ctx, tenantID, employeeID)
			if err == nil && managerEmployeeID != "" {
				selfEmployeeID, err := s.Employees.EmployeeIDByUserID(ctx, tenantID, approverUserID)
				if err == nil && selfEmployeeID != managerEmployeeID {
					return result, ErrForbidden
				}
			}
		}
	}

	finalApproval := !requiresHR || roleName == auth.RoleHR
	nextStatus := StatusApproved
	if requiresHR && roleName != auth.RoleHR {
		nextStatus = StatusPendingHR
		finalApproval = false
	}
	result.FinalApproval = finalApproval
	result.Status = nextStatus

	if err := s.Store.UpdateRequestStatus(ctx, requestID, nextStatus, approverUserID); err != nil {
		return result, err
	}

	if err := s.Store.InsertApproval(ctx, tenantID, requestID, approverUserID, "approved"); err != nil {
		return result, err
	}

	if finalApproval {
		if err := s.Store.UpdateBalanceOnApproval(ctx, tenantID, employeeID, leaveTypeID, days); err != nil {
			return result, err
		}
	} else {
		if hrUserIDs, err := s.Store.HRUserIDs(ctx, tenantID); err == nil {
			result.HRUserIDs = hrUserIDs
		}
	}

	if employeeUser, leaveTypeName, err := s.Store.EmployeeUserAndLeaveType(ctx, tenantID, leaveTypeID, employeeID); err == nil {
		result.EmployeeUser = employeeUser
		result.LeaveTypeName = leaveTypeName
	} else {
		result.EmployeeUser = ""
	}

	return result, nil
}

type RejectResult struct {
	EmployeeID    string
	LeaveTypeID   string
	LeaveTypeName string
	EmployeeUser  string
}

func (s *Service) RejectRequest(ctx context.Context, tenantID, requestID, approverUserID, roleName string) (RejectResult, error) {
	employeeID, leaveTypeID, days, _, err := s.Store.RequestInfo(ctx, tenantID, requestID)
	if err != nil {
		return RejectResult{}, err
	}
	result := RejectResult{EmployeeID: employeeID, LeaveTypeID: leaveTypeID}

	if roleName == auth.RoleManager {
		if s.Employees != nil {
			managerEmployeeID, err := s.Employees.ManagerIDByEmployeeID(ctx, tenantID, employeeID)
			if err == nil && managerEmployeeID != "" {
				selfEmployeeID, err := s.Employees.EmployeeIDByUserID(ctx, tenantID, approverUserID)
				if err == nil && selfEmployeeID != managerEmployeeID {
					return RejectResult{}, ErrForbidden
				}
			}
		}
	}

	if err := s.Store.UpdateRequestStatus(ctx, requestID, StatusRejected, approverUserID); err != nil {
		return RejectResult{}, err
	}

	if err := s.Store.InsertApproval(ctx, tenantID, requestID, approverUserID, "rejected"); err != nil {
		return RejectResult{}, err
	}

	if err := s.Store.UpdateBalanceOnReject(ctx, tenantID, employeeID, leaveTypeID, days); err != nil {
		return RejectResult{}, err
	}

	if employeeUser, leaveTypeName, err := s.Store.EmployeeUserAndLeaveType(ctx, tenantID, leaveTypeID, employeeID); err == nil {
		result.EmployeeUser = employeeUser
		result.LeaveTypeName = leaveTypeName
	} else {
		result.EmployeeUser = ""
	}
	return result, nil
}

type CancelResult struct {
	EmployeeID    string
	LeaveTypeID   string
	LeaveTypeName string
	EmployeeUser  string
}

func (s *Service) CancelRequest(ctx context.Context, tenantID, requestID, actorUserID string) (CancelResult, error) {
	employeeID, leaveTypeID, days, status, err := s.Store.RequestInfo(ctx, tenantID, requestID)
	if err != nil {
		return CancelResult{}, err
	}
	if status == StatusApproved {
		return CancelResult{}, ErrInvalidState
	}
	result := CancelResult{EmployeeID: employeeID, LeaveTypeID: leaveTypeID}

	if err := s.Store.UpdateRequestStatusSimple(ctx, requestID, StatusCancelled); err != nil {
		return CancelResult{}, err
	}

	if err := s.Store.UpdateBalanceOnReject(ctx, tenantID, employeeID, leaveTypeID, days); err != nil {
		return CancelResult{}, err
	}

	if employeeUser, leaveTypeName, err := s.Store.EmployeeUserAndLeaveType(ctx, tenantID, leaveTypeID, employeeID); err == nil {
		result.EmployeeUser = employeeUser
		result.LeaveTypeName = leaveTypeName
	} else {
		result.EmployeeUser = ""
	}
	return result, nil
}

type CalendarEntry struct {
	ID          string
	EmployeeID  string
	LeaveTypeID string
	StartDate   time.Time
	EndDate     time.Time
	Status      string
}

type CalendarExportRow struct {
	ID            string
	EmployeeID    string
	LeaveTypeName string
	StartDate     time.Time
	EndDate       time.Time
	Status        string
}

func (s *Service) CalendarEntries(ctx context.Context, tenantID, roleName, userID string) ([]CalendarEntry, error) {
	employeeID := ""
	if roleName == auth.RoleEmployee || roleName == auth.RoleManager {
		if id, err := s.EmployeeIDByUserID(ctx, tenantID, userID); err == nil {
			employeeID = id
		}
	}
	statuses := []string{StatusPending, StatusPendingHR, StatusApproved}
	return s.Store.CalendarEntries(ctx, tenantID, statuses, employeeID)
}

func (s *Service) CalendarExportRows(ctx context.Context, tenantID, roleName, userID string) ([]CalendarExportRow, error) {
	statuses := []string{StatusPending, StatusPendingHR, StatusApproved}
	employeeID := ""
	managerID := ""
	if roleName == auth.RoleEmployee {
		if id, err := s.EmployeeIDByUserID(ctx, tenantID, userID); err == nil {
			employeeID = id
		}
	}
	if roleName == auth.RoleManager {
		if id, err := s.EmployeeIDByUserID(ctx, tenantID, userID); err == nil {
			managerID = id
		}
	}
	return s.Store.CalendarExportRows(ctx, tenantID, statuses, employeeID, managerID)
}

func (s *Service) ReportBalances(ctx context.Context, tenantID string) ([]map[string]any, error) {
	return s.Store.ReportBalances(ctx, tenantID)
}

func (s *Service) ReportUsage(ctx context.Context, tenantID string) ([]map[string]any, error) {
	return s.Store.ReportUsage(ctx, tenantID)
}
