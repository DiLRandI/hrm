package leave

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hrm/internal/domain/auth"
)

func (s *Store) ListTypes(ctx context.Context, tenantID string) ([]LeaveType, error) {
	rows, err := s.DB.Query(ctx, `
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

func (s *Store) CreateType(ctx context.Context, tenantID string, payload LeaveType) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO leave_types (tenant_id, name, code, is_paid, requires_doc)
    VALUES ($1,$2,$3,$4,$5)
    RETURNING id
  `, tenantID, payload.Name, payload.Code, payload.IsPaid, payload.RequiresDoc).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) LeaveTypeRequiresDoc(ctx context.Context, tenantID, leaveTypeID string) (bool, error) {
	var requiresDoc bool
	if err := s.DB.QueryRow(ctx, `
    SELECT requires_doc
    FROM leave_types
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, leaveTypeID).Scan(&requiresDoc); err != nil {
		return false, err
	}
	return requiresDoc, nil
}

func (s *Store) ListPolicies(ctx context.Context, tenantID string) ([]LeavePolicy, error) {
	rows, err := s.DB.Query(ctx, `
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

func (s *Store) CreatePolicy(ctx context.Context, tenantID string, payload LeavePolicy) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO leave_policies (tenant_id, leave_type_id, accrual_rate, accrual_period, entitlement, carry_over_limit, allow_negative, requires_hr_approval)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    RETURNING id
  `, tenantID, payload.LeaveTypeID, payload.AccrualRate, payload.AccrualPeriod, payload.Entitlement, payload.CarryOver, payload.AllowNegative, payload.RequiresHRApproval).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) ListHolidays(ctx context.Context, tenantID string) ([]map[string]any, error) {
	rows, err := s.DB.Query(ctx, `
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

func (s *Store) CreateHoliday(ctx context.Context, tenantID string, date time.Time, name, region string) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO holidays (tenant_id, date, name, region)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, date, name, region).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) DeleteHoliday(ctx context.Context, tenantID, holidayID string) error {
	_, err := s.DB.Exec(ctx, "DELETE FROM holidays WHERE tenant_id = $1 AND id = $2", tenantID, holidayID)
	return err
}

func (s *Store) ListBalances(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	rows, err := s.DB.Query(ctx, `
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

func (s *Store) AdjustBalance(ctx context.Context, tenantID, employeeID, leaveTypeID, reason, userID string, amount float64) error {
	if _, err := s.DB.Exec(ctx, `
    INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
    VALUES ($1,$2,$3,$4,0,0)
    ON CONFLICT (employee_id, leave_type_id) DO UPDATE SET balance = leave_balances.balance + EXCLUDED.balance, updated_at = now()
  `, tenantID, employeeID, leaveTypeID, amount); err != nil {
		return err
	}

	if _, err := s.DB.Exec(ctx, `
    INSERT INTO leave_balance_adjustments (tenant_id, employee_id, leave_type_id, amount, reason, created_by)
    VALUES ($1,$2,$3,$4,$5,$6)
  `, tenantID, employeeID, leaveTypeID, amount, reason, userID); err != nil {
		return err
	}
	return nil
}

func (s *Store) ListRequests(ctx context.Context, tenantID, roleName, employeeID, managerEmployeeID string, limit, offset int) (RequestListResult, error) {
	query := `
    SELECT id, employee_id, leave_type_id, start_date, end_date, start_half, end_half, days, reason, status, created_at
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
	if err := s.DB.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		total = 0
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", limitPos, offsetPos)
	args = append(args, limit, offset)

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return RequestListResult{}, err
	}
	defer rows.Close()

	var requests []LeaveRequest
	for rows.Next() {
		var req LeaveRequest
		if err := rows.Scan(&req.ID, &req.EmployeeID, &req.LeaveTypeID, &req.StartDate, &req.EndDate, &req.StartHalf, &req.EndHalf, &req.Days, &req.Reason, &req.Status, &req.CreatedAt); err != nil {
			return RequestListResult{}, err
		}
		requests = append(requests, req)
	}

	requestIDs := make([]string, 0, len(requests))
	for _, req := range requests {
		requestIDs = append(requestIDs, req.ID)
	}
	if len(requestIDs) > 0 {
		docsByRequest, err := s.ListRequestDocuments(ctx, tenantID, requestIDs)
		if err != nil {
			return RequestListResult{}, err
		}
		for i := range requests {
			requests[i].Documents = docsByRequest[requests[i].ID]
		}
	}
	return RequestListResult{Requests: requests, Total: total}, nil
}

func (s *Store) GetRequest(ctx context.Context, tenantID, requestID string) (LeaveRequest, error) {
	var req LeaveRequest
	if err := s.DB.QueryRow(ctx, `
    SELECT id, employee_id, leave_type_id, start_date, end_date, start_half, end_half, days, reason, status, created_at
    FROM leave_requests
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, requestID).Scan(
		&req.ID,
		&req.EmployeeID,
		&req.LeaveTypeID,
		&req.StartDate,
		&req.EndDate,
		&req.StartHalf,
		&req.EndHalf,
		&req.Days,
		&req.Reason,
		&req.Status,
		&req.CreatedAt,
	); err != nil {
		return LeaveRequest{}, err
	}

	docsByRequest, err := s.ListRequestDocuments(ctx, tenantID, []string{requestID})
	if err != nil {
		return LeaveRequest{}, err
	}
	req.Documents = docsByRequest[requestID]
	return req, nil
}

func (s *Store) RequiresHRApproval(ctx context.Context, tenantID, leaveTypeID string) (bool, error) {
	var requiresHR bool
	if err := s.DB.QueryRow(ctx, `
    SELECT COALESCE(requires_hr_approval, false)
    FROM leave_policies
    WHERE tenant_id = $1 AND leave_type_id = $2
    LIMIT 1
  `, tenantID, leaveTypeID).Scan(&requiresHR); err != nil {
		return false, err
	}
	return requiresHR, nil
}

func (s *Store) CreateRequest(ctx context.Context, tenantID, employeeID, leaveTypeID, reason string, startDate, endDate time.Time, startHalf, endHalf bool, days float64, status string) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO leave_requests (tenant_id, employee_id, leave_type_id, start_date, end_date, start_half, end_half, days, reason, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
    RETURNING id
  `, tenantID, employeeID, leaveTypeID, startDate, endDate, startHalf, endHalf, days, reason, status).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) CreateRequestDocument(ctx context.Context, tenantID, requestID string, payload LeaveRequestDocumentUpload, uploadedBy string) (LeaveRequestDocument, error) {
	var out LeaveRequestDocument
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO leave_request_documents (tenant_id, leave_request_id, file_name, content_type, file_size, file_data, uploaded_by)
    VALUES ($1,$2,$3,$4,$5,$6,$7)
    RETURNING id, leave_request_id, file_name, content_type, file_size, uploaded_by, created_at
  `, tenantID, requestID, payload.FileName, payload.ContentType, payload.FileSize, payload.Data, uploadedBy).Scan(
		&out.ID,
		&out.LeaveRequestID,
		&out.FileName,
		&out.ContentType,
		&out.FileSize,
		&out.UploadedBy,
		&out.CreatedAt,
	); err != nil {
		return LeaveRequestDocument{}, err
	}
	return out, nil
}

func (s *Store) ListRequestDocuments(ctx context.Context, tenantID string, requestIDs []string) (map[string][]LeaveRequestDocument, error) {
	out := map[string][]LeaveRequestDocument{}
	if len(requestIDs) == 0 {
		return out, nil
	}

	placeholders := make([]string, 0, len(requestIDs))
	args := make([]any, 0, len(requestIDs)+1)
	args = append(args, tenantID)
	for idx, requestID := range requestIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx+2))
		args = append(args, requestID)
	}

	query := fmt.Sprintf(`
    SELECT id, leave_request_id, file_name, content_type, file_size, uploaded_by, created_at
    FROM leave_request_documents
    WHERE tenant_id = $1 AND leave_request_id IN (%s)
    ORDER BY created_at ASC
  `, strings.Join(placeholders, ","))

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var doc LeaveRequestDocument
		if err := rows.Scan(&doc.ID, &doc.LeaveRequestID, &doc.FileName, &doc.ContentType, &doc.FileSize, &doc.UploadedBy, &doc.CreatedAt); err != nil {
			return nil, err
		}
		out[doc.LeaveRequestID] = append(out[doc.LeaveRequestID], doc)
	}
	return out, nil
}

func (s *Store) RequestDocumentData(ctx context.Context, tenantID, requestID, documentID string) (LeaveRequestDocument, []byte, error) {
	var doc LeaveRequestDocument
	var data []byte
	if err := s.DB.QueryRow(ctx, `
    SELECT id, leave_request_id, file_name, content_type, file_size, uploaded_by, created_at, file_data
    FROM leave_request_documents
    WHERE tenant_id = $1 AND leave_request_id = $2 AND id = $3
  `, tenantID, requestID, documentID).Scan(
		&doc.ID,
		&doc.LeaveRequestID,
		&doc.FileName,
		&doc.ContentType,
		&doc.FileSize,
		&doc.UploadedBy,
		&doc.CreatedAt,
		&data,
	); err != nil {
		return LeaveRequestDocument{}, nil, err
	}
	return doc, data, nil
}

func (s *Store) AddPendingBalance(ctx context.Context, tenantID, employeeID, leaveTypeID string, days float64) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO leave_balances (tenant_id, employee_id, leave_type_id, balance, pending, used)
    VALUES ($1,$2,$3,0,$4,0)
    ON CONFLICT (employee_id, leave_type_id) DO UPDATE SET pending = leave_balances.pending + EXCLUDED.pending, updated_at = now()
  `, tenantID, employeeID, leaveTypeID, days)
	return err
}

func (s *Store) ManagerUserIDForEmployee(ctx context.Context, tenantID, employeeID string) (string, error) {
	var managerUserID string
	if err := s.DB.QueryRow(ctx, `
    SELECT m.user_id
    FROM employees e
    JOIN employees m ON e.manager_id = m.id
    WHERE e.tenant_id = $1 AND e.id = $2
  `, tenantID, employeeID).Scan(&managerUserID); err != nil {
		return "", err
	}
	return managerUserID, nil
}

func (s *Store) InsertApproval(ctx context.Context, tenantID, requestID, approverID, status string) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO leave_approvals (tenant_id, leave_request_id, approver_id, status)
    VALUES ($1,$2,$3,$4)
  `, tenantID, requestID, approverID, status)
	return err
}

func (s *Store) UpdateRequestStatus(ctx context.Context, requestID, status, approverID string) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE leave_requests SET status = $1, approved_by = $2, approved_at = now() WHERE id = $3
  `, status, approverID, requestID)
	return err
}

func (s *Store) UpdateRequestStatusSimple(ctx context.Context, requestID, status string) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE leave_requests SET status = $1 WHERE id = $2
  `, status, requestID)
	return err
}

func (s *Store) HRUserIDs(ctx context.Context, tenantID string) ([]string, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT u.id
    FROM users u
    JOIN roles r ON u.role_id = r.id
    WHERE u.tenant_id = $1 AND r.name = $2
  `, tenantID, auth.RoleHR)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hrUserIDs []string
	for rows.Next() {
		var hrUserID string
		if err := rows.Scan(&hrUserID); err == nil && hrUserID != "" {
			hrUserIDs = append(hrUserIDs, hrUserID)
		}
	}
	return hrUserIDs, nil
}

func (s *Store) RequestInfo(ctx context.Context, tenantID, requestID string) (string, string, float64, string, error) {
	var employeeID, leaveTypeID, status string
	var days float64
	if err := s.DB.QueryRow(ctx, `
    SELECT employee_id, leave_type_id, days, status
    FROM leave_requests
    WHERE id = $1 AND tenant_id = $2
  `, requestID, tenantID).Scan(&employeeID, &leaveTypeID, &days, &status); err != nil {
		return "", "", 0, "", err
	}
	return employeeID, leaveTypeID, days, status, nil
}

func (s *Store) UpdateBalanceOnApproval(ctx context.Context, tenantID, employeeID, leaveTypeID string, days float64) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE leave_balances
    SET pending = pending - $1, used = used + $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, tenantID, employeeID, leaveTypeID)
	return err
}

func (s *Store) UpdateBalanceOnReject(ctx context.Context, tenantID, employeeID, leaveTypeID string, days float64) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE leave_balances
    SET pending = pending - $1, updated_at = now()
    WHERE tenant_id = $2 AND employee_id = $3 AND leave_type_id = $4
  `, days, tenantID, employeeID, leaveTypeID)
	return err
}

func (s *Store) EmployeeUserAndLeaveType(ctx context.Context, tenantID, leaveTypeID, employeeID string) (string, string, error) {
	var employeeUser, leaveTypeName string
	if err := s.DB.QueryRow(ctx, `
    SELECT e.user_id, lt.name
    FROM employees e
    JOIN leave_types lt ON lt.id = $2
    WHERE e.tenant_id = $1 AND e.id = $3
  `, tenantID, leaveTypeID, employeeID).Scan(&employeeUser, &leaveTypeName); err != nil {
		return "", "", err
	}
	return employeeUser, leaveTypeName, nil
}

func (s *Store) CalendarEntries(ctx context.Context, tenantID string, statuses []string, employeeID string) ([]CalendarEntry, error) {
	query := `
    SELECT r.id, r.employee_id, e.first_name, e.last_name, r.leave_type_id, r.start_date, r.end_date, r.status
    FROM leave_requests r
    JOIN employees e ON r.employee_id = e.id
    WHERE r.tenant_id = $1 AND r.status IN ($2,$3,$4)
  `
	args := []any{tenantID, statuses[0], statuses[1], statuses[2]}
	if employeeID != "" {
		query += " AND (r.employee_id = $5 OR r.employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $5))"
		args = append(args, employeeID)
	}
	query += " ORDER BY r.start_date"

	rows, err := s.DB.Query(ctx, query, args...)
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

func (s *Store) CalendarExportRows(ctx context.Context, tenantID string, statuses []string, employeeID string, managerID string) ([]CalendarExportRow, error) {
	query := `
    SELECT lr.id, lr.employee_id, lt.name, lr.start_date, lr.end_date, lr.status
    FROM leave_requests lr
    JOIN leave_types lt ON lr.leave_type_id = lt.id
    WHERE lr.tenant_id = $1 AND lr.status IN ($2,$3,$4)
  `
	args := []any{tenantID, statuses[0], statuses[1], statuses[2]}
	if employeeID != "" {
		query += " AND lr.employee_id = $5"
		args = append(args, employeeID)
	}
	if managerID != "" {
		query += " AND lr.employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $5)"
		args = append(args, managerID)
	}
	query += " ORDER BY lr.start_date"

	rows, err := s.DB.Query(ctx, query, args...)
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

func (s *Store) ReportBalances(ctx context.Context, tenantID string) ([]map[string]any, error) {
	rows, err := s.DB.Query(ctx, `
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

func (s *Store) ReportUsage(ctx context.Context, tenantID string) ([]map[string]any, error) {
	rows, err := s.DB.Query(ctx, `
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
