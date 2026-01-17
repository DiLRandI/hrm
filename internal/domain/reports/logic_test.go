package reports

import "testing"

func TestEmployeeDashboard(t *testing.T) {
	payload := EmployeeDashboard(10.5, 2, 3)
	if payload["leaveBalance"].(float64) != 10.5 {
		t.Fatal("unexpected leave balance")
	}
	if payload["payslipCount"].(int) != 2 {
		t.Fatal("unexpected payslip count")
	}
	if payload["goalCount"].(int) != 3 {
		t.Fatal("unexpected goal count")
	}
}

func TestManagerDashboard(t *testing.T) {
	payload := ManagerDashboard(1, 4, 2)
	if payload["pendingApprovals"].(int) != 1 {
		t.Fatal("unexpected approvals count")
	}
	if payload["teamGoals"].(int) != 4 {
		t.Fatal("unexpected team goals")
	}
	if payload["reviewTasks"].(int) != 2 {
		t.Fatal("unexpected review tasks")
	}
}

func TestHRDashboard(t *testing.T) {
	payload := HRDashboard(3, 5, 7)
	if payload["payrollPeriods"].(int) != 3 {
		t.Fatal("unexpected payroll periods")
	}
	if payload["leavePending"].(int) != 5 {
		t.Fatal("unexpected leave pending")
	}
	if payload["reviewCycles"].(int) != 7 {
		t.Fatal("unexpected review cycles")
	}
}
