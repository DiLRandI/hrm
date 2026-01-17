package gdpr

func BuildDSARPayload(employee map[string]interface{}, leaveRequests, payrollResults, goals []map[string]interface{}) map[string]interface{} {
  return map[string]interface{}{
    "employee":       employee,
    "leaveRequests":  leaveRequests,
    "payrollResults": payrollResults,
    "goals":          goals,
  }
}
