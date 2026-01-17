package reports

func EmployeeDashboard(leaveBalance float64, payslipCount, goalCount int) map[string]interface{} {
  return map[string]interface{}{
    "leaveBalance": leaveBalance,
    "payslipCount": payslipCount,
    "goalCount":    goalCount,
  }
}

func ManagerDashboard(pendingApprovals, teamGoals, reviewTasks int) map[string]interface{} {
  return map[string]interface{}{
    "pendingApprovals": pendingApprovals,
    "teamGoals":        teamGoals,
    "reviewTasks":      reviewTasks,
  }
}

func HRDashboard(payrollPeriods, leavePending, reviewCycles int) map[string]interface{} {
  return map[string]interface{}{
    "payrollPeriods": payrollPeriods,
    "leavePending":   leavePending,
    "reviewCycles":   reviewCycles,
  }
}
