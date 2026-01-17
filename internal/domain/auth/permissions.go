package auth

const (
	PermEmployeesRead       = "core.employees.read"
	PermEmployeesWrite      = "core.employees.write"
	PermOrgRead             = "core.org.read"
	PermOrgWrite            = "core.org.write"
	PermLeaveRead           = "leave.read"
	PermLeaveWrite          = "leave.write"
	PermLeaveApprove        = "leave.approve"
	PermPayrollRead         = "payroll.read"
	PermPayrollWrite        = "payroll.write"
	PermPayrollRun          = "payroll.run"
	PermPayrollFinalize     = "payroll.finalize"
	PermPerformanceRead     = "performance.read"
	PermPerformanceWrite    = "performance.write"
	PermPerformanceReview   = "performance.review"
	PermPerformanceFinalize = "performance.finalize"
	PermReportsRead         = "reports.read"
	PermGDPRExport          = "gdpr.export"
	PermGDPRRetention       = "gdpr.retention"
	PermAuditRead           = "audit.read"
	PermSystemAdmin         = "admin.system"
)

var DefaultPermissions = []string{
	PermEmployeesRead,
	PermEmployeesWrite,
	PermOrgRead,
	PermOrgWrite,
	PermLeaveRead,
	PermLeaveWrite,
	PermLeaveApprove,
	PermPayrollRead,
	PermPayrollWrite,
	PermPayrollRun,
	PermPayrollFinalize,
	PermPerformanceRead,
	PermPerformanceWrite,
	PermPerformanceReview,
	PermPerformanceFinalize,
	PermReportsRead,
	PermGDPRExport,
	PermGDPRRetention,
	PermAuditRead,
	PermSystemAdmin,
}

var RolePermissions = map[string][]string{
	RoleEmployee: {
		PermEmployeesRead,
		PermOrgRead,
		PermLeaveRead,
		PermLeaveWrite,
		PermPayrollRead,
		PermPerformanceRead,
		PermPerformanceWrite,
		PermReportsRead,
	},
	RoleManager: {
		PermEmployeesRead,
		PermOrgRead,
		PermLeaveRead,
		PermLeaveWrite,
		PermLeaveApprove,
		PermPayrollRead,
		PermPerformanceRead,
		PermPerformanceWrite,
		PermPerformanceReview,
		PermReportsRead,
	},
	RoleHR: {
		PermEmployeesRead,
		PermEmployeesWrite,
		PermOrgRead,
		PermOrgWrite,
		PermLeaveRead,
		PermLeaveWrite,
		PermLeaveApprove,
		PermPayrollRead,
		PermPayrollWrite,
		PermPayrollRun,
		PermPayrollFinalize,
		PermPerformanceRead,
		PermPerformanceWrite,
		PermPerformanceReview,
		PermPerformanceFinalize,
		PermReportsRead,
		PermGDPRExport,
		PermGDPRRetention,
		PermAuditRead,
	},
	RoleSystemAdmin: {
		PermSystemAdmin,
	},
}
