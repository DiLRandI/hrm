package gdpr

func BuildDSARPayload(employee map[string]any, leaveRequests, payrollResults, goals []map[string]any) map[string]any {
	return map[string]any{
		"employee":       employee,
		"leaveRequests":  leaveRequests,
		"payrollResults": payrollResults,
		"goals":          goals,
	}
}
