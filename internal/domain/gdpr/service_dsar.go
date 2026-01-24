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

	datasets := map[string]any{}
	if rows, err := s.store.DSARLeaveRequests(ctx, tenantID, employeeID); err == nil {
		datasets["leaveRequests"] = rows
	}
	if rows, err := s.store.DSARPayrollResults(ctx, tenantID, employeeID); err == nil {
		datasets["payrollResults"] = rows
	}
	if rows, err := s.store.DSARPayslips(ctx, tenantID, employeeID); err == nil {
		datasets["payslips"] = rows
	}
	if rows, err := s.store.DSARGoals(ctx, tenantID, employeeID); err == nil {
		datasets["goals"] = rows
	}
	if rows, err := s.store.DSARGoalComments(ctx, tenantID, employeeID); err == nil {
		datasets["goalComments"] = rows
	}
	if rows, err := s.store.DSARFeedback(ctx, tenantID, employeeID, emp.UserID); err == nil {
		datasets["feedback"] = rows
	}
	if rows, err := s.store.DSARCheckins(ctx, tenantID, employeeID); err == nil {
		datasets["checkins"] = rows
	}
	if rows, err := s.store.DSARPIPs(ctx, tenantID, employeeID); err == nil {
		datasets["pips"] = rows
	}
	if rows, err := s.store.DSARReviewTasks(ctx, tenantID, employeeID); err == nil {
		datasets["reviewTasks"] = rows
	}
	if rows, err := s.store.DSARReviewResponses(ctx, tenantID, employeeID); err == nil {
		datasets["reviewResponses"] = rows
	}
	if rows, err := s.store.DSARConsentRecords(ctx, tenantID, employeeID); err == nil {
		datasets["consentRecords"] = rows
	}
	if emp.UserID != "" {
		if rows, err := s.store.DSARNotifications(ctx, tenantID, emp.UserID); err == nil {
			datasets["notifications"] = rows
		}
	}
	if rows, err := s.store.DSARAccessLogs(ctx, tenantID, employeeID); err == nil {
		datasets["accessLogs"] = rows
	}
	if rows, err := s.store.DSARManagerHistory(ctx, employeeID); err == nil {
		datasets["managerHistory"] = rows
	}
	if rows, err := s.store.DSAREmergencyContacts(ctx, tenantID, employeeID); err == nil {
		datasets["emergencyContacts"] = rows
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
