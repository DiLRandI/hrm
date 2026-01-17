package gdpr

import "testing"

func TestBuildDSARPayload(t *testing.T) {
	employee := map[string]any{"id": "e1"}
	datasets := map[string]any{
		"leaveRequests":  []map[string]any{{"id": "l1"}},
		"payrollResults": []map[string]any{{"id": "p1"}},
		"goals":          []map[string]any{{"id": "g1"}},
	}

	payload := BuildDSARPayload(employee, datasets)

	if payload["employee"] == nil {
		t.Fatal("expected employee in payload")
	}
	if len(payload["leaveRequests"].([]map[string]any)) != 1 {
		t.Fatal("expected leave requests")
	}
	if len(payload["payrollResults"].([]map[string]any)) != 1 {
		t.Fatal("expected payroll results")
	}
	if len(payload["goals"].([]map[string]any)) != 1 {
		t.Fatal("expected goals")
	}
}
