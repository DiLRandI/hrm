package leave

import (
	"errors"
	"time"
)

// CalculateDays returns inclusive day count between start and end.
func CalculateDays(start, end time.Time) (float64, error) {
	if end.Before(start) {
		return 0, errors.New("end date before start date")
	}
	return end.Sub(start).Hours()/24 + 1, nil
}

// CalculateRequestDays returns inclusive leave day count with optional half-day start/end boundaries.
func CalculateRequestDays(start, end time.Time, startHalf, endHalf bool) (float64, error) {
	days, err := CalculateDays(start, end)
	if err != nil {
		return 0, err
	}

	sameDay := start.Equal(end)
	if sameDay && startHalf && endHalf {
		return 0, errors.New("invalid half-day range")
	}

	if startHalf {
		days -= 0.5
	}
	if endHalf {
		days -= 0.5
	}
	if days <= 0 {
		return 0, errors.New("invalid half-day range")
	}
	return days, nil
}
