package gdpr

import "testing"

func TestBuildDSARPayload(t *testing.T) {
  employee := map[string]interface{}{"id": "e1"}
  leaves := []map[string]interface{}{{"id": "l1"}}
  payroll := []map[string]interface{}{{"id": "p1"}}
  goals := []map[string]interface{}{{"id": "g1"}}

  payload := BuildDSARPayload(employee, leaves, payroll, goals)

  if payload["employee"] == nil {
    t.Fatal("expected employee in payload")
  }
  if len(payload["leaveRequests"].([]map[string]interface{})) != 1 {
    t.Fatal("expected leave requests")
  }
  if len(payload["payrollResults"].([]map[string]interface{})) != 1 {
    t.Fatal("expected payroll results")
  }
  if len(payload["goals"].([]map[string]interface{})) != 1 {
    t.Fatal("expected goals")
  }
}
