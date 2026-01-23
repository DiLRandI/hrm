package gdpr

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func (s *Service) GenerateDSAR(ctx context.Context, tenantID, employeeID, exportID string) (string, bool, string, error) {
	if s.employees == nil {
		return "", false, "", errors.New("employee store unavailable")
	}
	if s.store == nil {
		return "", false, "", errors.New("store unavailable")
	}

	emp, err := s.employees.GetEmployee(ctx, tenantID, employeeID)
	if err != nil {
		return "", false, "", err
	}
	employeeJSON, err := json.Marshal(emp)
	if err != nil {
		return "", false, "", err
	}
	var employee map[string]any
	if err := json.Unmarshal(employeeJSON, &employee); err != nil {
		return "", false, "", err
	}

	queryRows := func(query string, args ...any) ([]map[string]any, error) {
		rows, err := s.store.DB.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []map[string]any
		for rows.Next() {
			var rowJSON []byte
			if err := rows.Scan(&rowJSON); err != nil {
				return nil, err
			}
			var row map[string]any
			if err := json.Unmarshal(rowJSON, &row); err != nil {
				return nil, err
			}
			out = append(out, row)
		}
		return out, rows.Err()
	}

	datasets := map[string]any{}
	if rows, err := queryRows(`SELECT row_to_json(lr) FROM leave_requests lr WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["leaveRequests"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(pr) FROM payroll_results pr WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["payrollResults"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(ps) FROM payslips ps WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["payslips"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(g) FROM goals g WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["goals"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(gc) FROM goal_comments gc JOIN goals g ON gc.goal_id = g.id WHERE g.tenant_id = $1 AND g.employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["goalComments"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(f) FROM feedback f WHERE tenant_id = $1 AND (to_employee_id = $2 OR from_user_id = $3)`, tenantID, employeeID, emp.UserID); err == nil {
		datasets["feedback"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(c) FROM checkins c WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["checkins"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(p) FROM pips p WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["pips"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(rt) FROM review_tasks rt WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["reviewTasks"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(rr) FROM review_responses rr JOIN review_tasks rt ON rr.task_id = rt.id WHERE rr.tenant_id = $1 AND rt.employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["reviewResponses"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(cr) FROM consent_records cr WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["consentRecords"] = rows
	}
	if emp.UserID != "" {
		if rows, err := queryRows(`SELECT row_to_json(n) FROM notifications n WHERE tenant_id = $1 AND user_id = $2`, tenantID, emp.UserID); err == nil {
			datasets["notifications"] = rows
		}
	}
	if rows, err := queryRows(`SELECT row_to_json(al) FROM access_logs al WHERE tenant_id = $1 AND employee_id = $2`, tenantID, employeeID); err == nil {
		datasets["accessLogs"] = rows
	}
	if rows, err := queryRows(`SELECT row_to_json(mr) FROM manager_relations mr WHERE mr.employee_id = $1`, employeeID); err == nil {
		datasets["managerHistory"] = rows
	}

	payload := BuildDSARPayload(employee, datasets)
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", false, "", err
	}

	if err := os.MkdirAll("storage/dsar", 0o755); err != nil {
		return "", false, "", err
	}

	encrypted := false
	filePath := filepath.Join("storage/dsar", exportID+".json")
	if s.crypto != nil && s.crypto.Configured() {
		enc, err := s.crypto.Encrypt(jsonBytes)
		if err != nil {
			return "", false, "", err
		}
		encrypted = true
		filePath = filePath + ".enc"
		if err := os.WriteFile(filePath, enc, 0o600); err != nil {
			return "", false, "", err
		}
	} else {
		if err := os.WriteFile(filePath, jsonBytes, 0o600); err != nil {
			return "", false, "", err
		}
	}

	token, err := generateDownloadToken()
	if err != nil {
		return "", encrypted, "", err
	}
	return filePath, encrypted, token, nil
}

func generateDownloadToken() (string, error) {
	buff := make([]byte, 32)
	if _, err := rand.Read(buff); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buff), nil
}
