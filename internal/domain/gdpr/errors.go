package gdpr

import "errors"

var (
	ErrAnonymizationNotFound = errors.New("anonymization job not found")
	ErrAnonymizationBadState = errors.New("anonymization job is not in requested state")
)
