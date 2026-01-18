package leave

import (
	"context"
	"errors"
	"fmt"
	"time"

	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
)

type Service struct {
	Store *Store
	Core  *core.Store
}

var (
	ErrHRApprovalRequired = errors.New("hr approval required")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidState       = errors.New("invalid state")
)

func NewService(store *Store, coreStore *core.Store) *Service {
	return &Service{Store: store, Core: coreStore}
}

func (s *Service) ListTypes(ctx context.Context, tenantID string) ([]LeaveType, error) {
	rows, err := s.Store.DB.Query(ctx, `
    SELECT id, name, code, is_paid, requires_doc, created_at
    FROM leave_types
    WHERE tenant_id = $1
    ORDER BY name
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []LeaveType
	for rows.Next() {
		var t LeaveType
		if err := rows.Scan(&t.ID, &t.Name, &t.Code, &t.IsPaid, &t.RequiresDoc, &t.CreatedAt); err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	return types, nil
}

func (s *Service) CreateType(ctx context.Context, tenantID string, payload LeaveType) (string, error) {
	var id string
	if err := s.Store.DB.QueryRow(ctx, `
    INSERT INTO leave_types (tenant_id, name, code, is_paid, requires_doc)
    VALUES ($1,$2,$3,$4,$5)
    RETURNING id
  `, tenantID, payload.Name, payload.Code, payload.IsPaid, payload.RequiresDoc).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Service) ListPolicies(ctx context.Context, tenantID string) ([]LeavePolicy, error) {
	rows, err := s.Store.DB.Query(ctx, `
    SELECT id, leave_type_id, accrual_rate, accrual_period, entitlement, carry_over_limit, allow_negative, requires_hr_approval
    FROM leave_policies
    WHERE tenant_id = $1
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []LeavePolicy
	for rows.Next() {
		var p LeavePolicy
		if err := rows.Scan(&p.ID, &p.LeaveTypeID, &p.AccrualRate, &p.AccrualPeriod, &p.Entitlement, &p.CarryOver, &p.AllowNegative, &p.RequiresHRApproval); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, nil
}

func (s *Service) CreatePolicy(ctx context.Context, tenantID string, payload LeavePolicy) (string, error) {
	var id string
	if err := s.Store.DB.QueryRow(ctx, `
    INSERT INTO leave_policies (tenant_id, leave_type_id, accrual_rate, accrual_period, entitlement, carry_over_limit, allow_negative, requires_hr_approval)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    RETURNING id
  `, tenantID, payload.LeaveTypeID, payload.AccrualRate, payload.AccrualPeriod, payload.Entitlement, payload.CarryOver, payload.AllowNegative, payload.RequiresHRApproval).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Service) ListHolidays(ctx context.Context, tenantID string) ([]map[string]any, error) {
	rows, err := s.Store.DB.Query(ctx, `
    SELECT id, date, name, region
    FROM holidays
    WHERE tenant_id = $1
    ORDER BY date
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, name, region string
		var date time.Time
		if err := rows.Scan(&id, &date, &name, &region); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id":     id,
			"date":   date,
			"name":   name,
			"region": region,
		})
	}
	return out, nil
}

func (s *Service) CreateHoliday(ctx context.Context, tenantID string, date time.Time, name, region string) (string, error) {
	var id string
	if err := s.Store.DB.QueryRow(ctx, `
    INSERT INTO holidays (tenant_id, date, name, region)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, date, name, region).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Service) DeleteHoliday(ctx context.Context, tenantID, holidayID string) error {
	_, err := s.Store.DB.Exec(ctx, "DELETE FROM holidays WHERE tenant_id = $1 AND id = $2", tenantID, holidayID)
	return err
}

func (s *Service) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	if s.Core == nil {
		return "", nil
	}
	return s.Core.EmployeeIDByUserID(ctx, tenantID, userID)
}

func (s *Service) IsManagerOf(ctx context.Context, tenantID, managerEmployeeID, employeeID string) (bool, error) {
	if s.Core == nil {
		return false, nil
	}
	return s.Core.IsManagerOf(ctx, tenantID, managerEmployeeID, employeeID)
}

func (s *Service) ListBalances(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	rows, err := s.Store.DB.Query(ctx, `
    SELECT id, employee_id, leave_type_id, balance, pending, used, updated_at
    FROM leave_balances
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balances []map[string]any
	for rows.Next() {
		var id, employee, leaveType string
		var balance, pending, used float64
		var updatedAt time.Time
		if err := rows.Scan(&id, &employee, &leaveType, &balance, &pending, &used, &updatedAt); err != nil {
			return nil, err
		}
		balances = append(balances, map[string]any{
			"id":          id,
			"employeeId":  employee,
			"leaveTypeId": leaveType,
			"balance":     balance,
			"pending":     pending,
			"used":        used,
			"updatedAt":   updatedAt,
		})
	}
	return balances, nil
}

func (s *Service) AdjustBalance(ctx context.Context, tenantID, employeeID, leaveTypeID, reason, userID string, amount float64) error {
	if _, err := s.Store.DB.Exec(ctx, `
    INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
    VALUES ($1,$2,$3,$4,0,0)
    ON CONFLICT (employee_id, leave_type_id) DO UPDATE SET balance = leave_balances.balance + EXCLUDED.balance, updated_at = now()
  `, tenantID, employeeID, leaveTypeID, amount); err != nil {
		return err
	}

	if _, err := s.Store.DB.Exec(ctx, `
    INSERT INTO leave_balance_adjustments (tenant_id, employee_id, leave_type_id, amount, reason, created_by)
    VALUES ($1,$2,$3,$4,$5,$6)
  `, tenantID, employeeID, leaveTypeID, amount, reason, userID); err != nil {
		return err
	}
	return nil
}

func (s *Service) RunAccruals(ctx context.Context, tenantID string, now time.Time) (AccrualSummary, error) {
	return ApplyAccruals(ctx, s.Store.DB, tenantID, now)
}

type RequestListResult struct {
	Requests []LeaveRequest
	Total    int
}

func (s *Service) ListRequests(ctx context.Context, tenantID, roleName, employeeID, managerEmployeeID string, limit, offset int) (RequestListResult, error) {
	query := `
    SELECT id, employee_id, leave_type_id, start_date, end_date, days, reason, status, created_at
    FROM leave_requests
    WHERE tenant_id = $1
  `
	args := []any{tenantID}

	if roleName == auth.RoleEmployee {
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	if roleName == auth.RoleManager {
		query += " AND employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $2)"
		args = append(args, managerEmployeeID)
	}
	query += " ORDER BY created_at DESC"

	countQuery := `
    SELECT COUNT(1)
    FROM leave_requests
    WHERE tenant_id = $1
  `
	countArgs := []any{tenantID}
	if roleName == auth.RoleEmployee {
		countQuery += " AND employee_id = $2"
		countArgs = append(countArgs, employeeID)
	}
	if roleName == auth.RoleManager {
		countQuery += " AND employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $2)"
		countArgs = append(countArgs, managerEmployeeID)
	}
	var total int
	if err := s.Store.DB.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		total = 0
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", limitPos, offsetPos)
	args = append(args, limit, offset)

	rows, err := s.Store.DB.Query(ctx, query, args...)
	if err != nil {
		return RequestListResult{}, err
	}
	defer rows.Close()

	var requests []LeaveRequest
	for rows.Next() {
		var req LeaveRequest
		if err := rows.Scan(&req.ID, &req.EmployeeID, &req.LeaveTypeID, &req.StartDate, &req.EndDate, &req.Days, &req.Reason, &req.Status, &req.CreatedAt); err != nil {
			return RequestListResult{}, err
		}
		requests = append(requests, req)
	}
	return RequestListResult{Requests: requests, Total: total}, nil
}

type CreateRequestResult struct {
	ID            string
	Status        string
	ManagerUserID string
	HRUserIDs     []string
}

func (s *Service) CreateRequest(ctx context.Context, tenantID, employeeID, leaveTypeID, reason string, startDate, endDate time.Time, days float64) (CreateRequestResult, error) {
	result := CreateRequestResult{Status: StatusPending}
	var requiresHR bool
	if err := s.Store.DB.QueryRow(ctx, `
    SELECT COALESCE(requires_hr_approval, false)
    FROM leave_policies
    WHERE tenant_id = $1 AND leave_type_id = $2
    LIMIT 1
  `, tenantID, leaveTypeID).Scan(&requiresHR); err != nil {
		requiresHR = false
	}

	if err := s.Store.DB.QueryRow(ctx, `
    INSERT INTO leave_requests (tenant_id, employee_id, leave_type_id, start_date, end_date, days, reason, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    RETURNING id
  `, tenantID, employeeID, leaveTypeID, startDate, endDate, days, reason, StatusPending).Scan(&result.ID); err != nil {
		return result, err
	}

	if _, err := s.Store.DB.Exec(ctx, `
    INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
    VALUES ($1,$2,$3,0,$4,0)
    ON CONFLICT (employee_id, leave_type_id) DO UPDATE SET pending = leave_balances.pending + EXCLUDED.pending, updated_at = now()
  `, tenantID, employeeID, leaveTypeID, days); err != nil {
		return result, err
	}

	if err := s.Store.DB.QueryRow(ctx, `
    SELECT m.user_id
    FROM employees e
    JOIN employees m ON e.manager_id = m.id
    WHERE e.tenant_id = $1 AND e.id = $2
  `, tenantID, employeeID).Scan(&result.ManagerUserID); err != nil {
		result.ManagerUserID = ""
	}
	if result.ManagerUserID == "" {
		requiresHR = true
	}

	if result.ManagerUserID != "" {
		if _, err := s.Store.DB.Exec(ctx, `
      INSERT INTO leave_approvals (tenant_id, leave_request_id, approver_id, status)
      VALUES ($1,$2,$3,$4)
    `, tenantID, result.ID, result.ManagerUserID, StatusPending); err != nil {
			return result, err
		}
	} else if requiresHR {
		if _, err := s.Store.DB.Exec(ctx, `
      UPDATE leave_requests SET status = $1 WHERE tenant_id = $2 AND id = $3
    `, StatusPendingHR, tenantID, result.ID); err != nil {
			return result, err
		}
		rows, err := s.Store.DB.Query(ctx, `
        SELECT u.id
        FROM users u
        JOIN roles r ON u.role_id = r.id
        WHERE u.tenant_id = $1 AND r.name = $2
      `, tenantID, auth.RoleHR)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var hrUserID string
				if err := rows.Scan(&hrUserID); err == nil && hrUserID != "" {
					result.HRUserIDs = append(result.HRUserIDs, hrUserID)
				}
			}
		}
	}

	if result.ManagerUserID == "" && requiresHR {
		result.Status = StatusPendingHR
	}

	return result, nil
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
	var employeeID, leaveTypeID, currentStatus string
	var days float64
	if err := s.Store.DB.QueryRow(ctx, `
    SELECT employee_id, leave_type_id, days, status
    FROM leave_requests
    WHERE id = $1 AND tenant_id = $2
  `, requestID, tenantID).Scan(&employeeID, &leaveTypeID, &days, &currentStatus); err != nil {
		return result, err
	}
	result.EmployeeID = employeeID
	result.LeaveTypeID = leaveTypeID

	var requiresHR bool
	if err := s.Store.DB.QueryRow(ctx, `
    SELECT COALESCE(requires_hr_approval,false)
    FROM leave_policies
    WHERE tenant_id = $1 AND leave_type_id = $2
    LIMIT 1
  `, tenantID, leaveTypeID).Scan(&requiresHR); err != nil {
		requiresHR = false
	}

	if currentStatus == StatusPendingHR && roleName != auth.RoleHR {
		return result, ErrHRApprovalRequired
	}

	if roleName == auth.RoleManager {
		var managerEmployeeID string
		if err := s.Store.DB.QueryRow(ctx, "SELECT manager_id FROM employees WHERE tenant_id = $1 AND id = $2", tenantID, employeeID).Scan(&managerEmployeeID); err == nil && managerEmployeeID != "" {
			var selfEmployeeID string
			if err := s.Store.DB.QueryRow(ctx, "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", tenantID, approverUserID).Scan(&selfEmployeeID); err == nil {
				if selfEmployeeID != managerEmployeeID {
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

	if _, err := s.Store.DB.Exec(ctx, `
    UPDATE leave_requests SET status = $1, approved_by = $2, approved_at = now() WHERE id = $3
  `, nextStatus, approverUserID, requestID); err != nil {
		return result, err
	}

	if _, err := s.Store.DB.Exec(ctx, `
    INSERT INTO leave_approvals (tenant_id, leave_request_id, approver_id, status, decided_at)
    VALUES ($1,$2,$3,$4,now())
  `, tenantID, requestID, approverUserID, "approved"); err != nil {
		return result, err
	}

	if finalApproval {
		if _, err := s.Store.DB.Exec(ctx, `
    UPDATE leave_balances
    SET pending = pending - $1, used = used + $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, tenantID, employeeID, leaveTypeID); err != nil {
			return result, err
		}
	} else {
		rows, err := s.Store.DB.Query(ctx, `
      SELECT u.id
      FROM users u
      JOIN roles r ON u.role_id = r.id
      WHERE u.tenant_id = $1 AND r.name = $2
    `, tenantID, auth.RoleHR)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var hrUserID string
				if err := rows.Scan(&hrUserID); err == nil && hrUserID != "" {
					result.HRUserIDs = append(result.HRUserIDs, hrUserID)
				}
			}
		}
	}

	if err := s.Store.DB.QueryRow(ctx, `
    SELECT e.user_id, lt.name
    FROM employees e
    JOIN leave_types lt ON lt.id = $2
    WHERE e.tenant_id = $1 AND e.id = $3
  `, tenantID, leaveTypeID, employeeID).Scan(&result.EmployeeUser, &result.LeaveTypeName); err != nil {
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
	var employeeID, leaveTypeID string
	var days float64
	if err := s.Store.DB.QueryRow(ctx, `
    SELECT employee_id, leave_type_id, days
    FROM leave_requests
    WHERE id = $1 AND tenant_id = $2
  `, requestID, tenantID).Scan(&employeeID, &leaveTypeID, &days); err != nil {
		return RejectResult{}, err
	}
	result := RejectResult{EmployeeID: employeeID, LeaveTypeID: leaveTypeID}

	if roleName == auth.RoleManager {
		var managerEmployeeID string
		if err := s.Store.DB.QueryRow(ctx, "SELECT manager_id FROM employees WHERE tenant_id = $1 AND id = $2", tenantID, employeeID).Scan(&managerEmployeeID); err == nil && managerEmployeeID != "" {
			var selfEmployeeID string
			if err := s.Store.DB.QueryRow(ctx, "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", tenantID, approverUserID).Scan(&selfEmployeeID); err == nil {
				if selfEmployeeID != managerEmployeeID {
					return RejectResult{}, ErrForbidden
				}
			}
		}
	}

	if _, err := s.Store.DB.Exec(ctx, `
    UPDATE leave_requests SET status = $1, approved_by = $2, approved_at = now() WHERE id = $3
  `, StatusRejected, approverUserID, requestID); err != nil {
		return RejectResult{}, err
	}

	if _, err := s.Store.DB.Exec(ctx, `
    INSERT INTO leave_approvals (tenant_id, leave_request_id, approver_id, status, decided_at)
    VALUES ($1,$2,$3,$4,now())
  `, tenantID, requestID, approverUserID, "rejected"); err != nil {
		return RejectResult{}, err
	}

	if _, err := s.Store.DB.Exec(ctx, `
    UPDATE leave_balances
    SET pending = pending - $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, tenantID, employeeID, leaveTypeID); err != nil {
		return RejectResult{}, err
	}

	if err := s.Store.DB.QueryRow(ctx, `
    SELECT e.user_id, lt.name
    FROM employees e
    JOIN leave_types lt ON lt.id = $2
    WHERE e.tenant_id = $1 AND e.id = $3
  `, tenantID, leaveTypeID, employeeID).Scan(&result.EmployeeUser, &result.LeaveTypeName); err != nil {
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
	var employeeID, leaveTypeID, status string
	var days float64
	if err := s.Store.DB.QueryRow(ctx, `
    SELECT employee_id, leave_type_id, days, status
    FROM leave_requests
    WHERE id = $1 AND tenant_id = $2
  `, requestID, tenantID).Scan(&employeeID, &leaveTypeID, &days, &status); err != nil {
		return CancelResult{}, err
	}
	if status == StatusApproved {
		return CancelResult{}, ErrInvalidState
	}
	result := CancelResult{EmployeeID: employeeID, LeaveTypeID: leaveTypeID}

	if _, err := s.Store.DB.Exec(ctx, `
    UPDATE leave_requests SET status = $1 WHERE id = $2
  `, StatusCancelled, requestID); err != nil {
		return CancelResult{}, err
	}

	if _, err := s.Store.DB.Exec(ctx, `
    UPDATE leave_balances
    SET pending = pending - $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, tenantID, employeeID, leaveTypeID); err != nil {
		return CancelResult{}, err
	}

	if err := s.Store.DB.QueryRow(ctx, `
    SELECT e.user_id, lt.name
    FROM employees e
    JOIN leave_types lt ON lt.id = $2
    WHERE e.tenant_id = $1 AND e.id = $3
  `, tenantID, leaveTypeID, employeeID).Scan(&result.EmployeeUser, &result.LeaveTypeName); err != nil {
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
	query := `
    SELECT r.id, r.employee_id, e.first_name, e.last_name, r.leave_type_id, r.start_date, r.end_date, r.status
    FROM leave_requests r
    JOIN employees e ON r.employee_id = e.id
    WHERE r.tenant_id = $1 AND r.status IN ($2,$3,$4)
  `
	args := []any{tenantID, StatusPending, StatusPendingHR, StatusApproved}
	if roleName == auth.RoleEmployee || roleName == auth.RoleManager {
		employeeID, err := s.EmployeeIDByUserID(ctx, tenantID, userID)
		if err == nil && employeeID != "" {
			query += " AND (r.employee_id = $5 OR r.employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $5))"
			args = append(args, employeeID)
		}
	}
	query += " ORDER BY r.start_date"

	rows, err := s.Store.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CalendarEntry
	for rows.Next() {
		var id, employeeID, first, last, leaveTypeID, status string
		var startDate, endDate time.Time
		if err := rows.Scan(&id, &employeeID, &first, &last, &leaveTypeID, &startDate, &endDate, &status); err != nil {
			return nil, err
		}
		out = append(out, CalendarEntry{
			ID:          id,
			EmployeeID:  employeeID,
			LeaveTypeID: leaveTypeID,
			StartDate:   startDate,
			EndDate:     endDate,
			Status:      status,
		})
	}
	return out, nil
}

func (s *Service) CalendarExportRows(ctx context.Context, tenantID, roleName, userID string) ([]CalendarExportRow, error) {
	query := `
    SELECT lr.id, lr.employee_id, lt.name, lr.start_date, lr.end_date, lr.status
    FROM leave_requests lr
    JOIN leave_types lt ON lr.leave_type_id = lt.id
    WHERE lr.tenant_id = $1 AND lr.status IN ($2,$3,$4)
  `
	args := []any{tenantID, StatusPending, StatusPendingHR, StatusApproved}
	if roleName == auth.RoleEmployee {
		employeeID, err := s.EmployeeIDByUserID(ctx, tenantID, userID)
		if err == nil && employeeID != "" {
			query += " AND lr.employee_id = $5"
			args = append(args, employeeID)
		}
	}
	if roleName == auth.RoleManager {
		managerEmployeeID, err := s.EmployeeIDByUserID(ctx, tenantID, userID)
		if err == nil && managerEmployeeID != "" {
			query += " AND lr.employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $5)"
			args = append(args, managerEmployeeID)
		}
	}
	query += " ORDER BY lr.start_date"

	rows, err := s.Store.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CalendarExportRow
	for rows.Next() {
		var id, employeeID, leaveTypeName, status string
		var startDate, endDate time.Time
		if err := rows.Scan(&id, &employeeID, &leaveTypeName, &startDate, &endDate, &status); err != nil {
			return nil, err
		}
		out = append(out, CalendarExportRow{
			ID:            id,
			EmployeeID:    employeeID,
			LeaveTypeName: leaveTypeName,
			StartDate:     startDate,
			EndDate:       endDate,
			Status:        status,
		})
	}
	return out, nil
}

func (s *Service) ReportBalances(ctx context.Context, tenantID string) ([]map[string]any, error) {
	rows, err := s.Store.DB.Query(ctx, `
    SELECT employee_id, leave_type_id, balance, pending, used
    FROM leave_balances
    WHERE tenant_id = $1
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var employeeID, leaveTypeID string
		var balance, pending, used float64
		if err := rows.Scan(&employeeID, &leaveTypeID, &balance, &pending, &used); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"employeeId":  employeeID,
			"leaveTypeId": leaveTypeID,
			"balance":     balance,
			"pending":     pending,
			"used":        used,
		})
	}
	return out, nil
}

func (s *Service) ReportUsage(ctx context.Context, tenantID string) ([]map[string]any, error) {
	rows, err := s.Store.DB.Query(ctx, `
    SELECT leave_type_id, SUM(days)
    FROM leave_requests
    WHERE tenant_id = $1 AND status = $2
    GROUP BY leave_type_id
  `, tenantID, StatusApproved)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var leaveTypeID string
		var total float64
		if err := rows.Scan(&leaveTypeID, &total); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"leaveTypeId": leaveTypeID,
			"totalDays":   total,
		})
	}
	return out, nil
}
