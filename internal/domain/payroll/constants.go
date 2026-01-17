package payroll

const (
	PeriodStatusDraft     = "draft"
	PeriodStatusReviewed  = "reviewed"
	PeriodStatusFinalized = "finalized"

	WarningMissingBank = "missing_bank_account"
	WarningNegativeNet = "negative_net"
	WarningNetVariance = "net_variance"

	InputSourceManual = "manual"
	InputSourceImport = "import"

	ElementTypeEarning   = "earning"
	ElementTypeDeduction = "deduction"
)
