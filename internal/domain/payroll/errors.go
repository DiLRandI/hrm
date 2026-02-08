package payroll

import "errors"

var (
	ErrPeriodNotFound       = errors.New("payroll period not found")
	ErrFinalizeInvalidState = errors.New("payroll period must be reviewed before finalize")
	ErrFinalizeNoResults    = errors.New("payroll period has no payroll results")
)
