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
