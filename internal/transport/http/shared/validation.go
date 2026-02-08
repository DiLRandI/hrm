package shared

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"hrm/internal/transport/http/api"
)

type ValidationIssue struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

type Validator struct {
	issues []ValidationIssue
}

func NewValidator() *Validator {
	return &Validator{issues: make([]ValidationIssue, 0, 4)}
}

func (v *Validator) Add(field, reason string) {
	if v == nil {
		return
	}
	field = strings.TrimSpace(field)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return
	}
	v.issues = append(v.issues, ValidationIssue{
		Field:  field,
		Reason: reason,
	})
}

func (v *Validator) Required(field, value, reason string) {
	if strings.TrimSpace(value) == "" {
		v.Add(field, reason)
	}
}

func (v *Validator) Enum(field, value string, allowed []string, reason string) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return
	}
	for _, candidate := range allowed {
		if normalized == strings.ToLower(strings.TrimSpace(candidate)) {
			return
		}
	}
	v.Add(field, reason)
}

func (v *Validator) Date(field, raw string) (time.Time, bool) {
	parsed, err := ParseDate(strings.TrimSpace(raw))
	if err != nil || parsed.IsZero() {
		v.Add(field, "must be a valid date in YYYY-MM-DD format")
		return time.Time{}, false
	}
	return parsed, true
}

func (v *Validator) DateOrder(startField string, start time.Time, endField string, end time.Time) {
	if start.IsZero() || end.IsZero() {
		return
	}
	if end.Before(start) {
		v.Add(startField, "must be on or before "+endField)
		v.Add(endField, "must be on or after "+startField)
	}
}

func (v *Validator) HasIssues() bool {
	return v != nil && len(v.issues) > 0
}

func (v *Validator) Issues() []ValidationIssue {
	if v == nil || len(v.issues) == 0 {
		return nil
	}
	out := make([]ValidationIssue, len(v.issues))
	copy(out, v.issues)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Field == out[j].Field {
			return out[i].Reason < out[j].Reason
		}
		return out[i].Field < out[j].Field
	})
	return out
}

func (v *Validator) Reject(w http.ResponseWriter, requestID string) bool {
	if !v.HasIssues() {
		return false
	}
	FailValidation(w, requestID, v.Issues())
	return true
}

func FailValidation(w http.ResponseWriter, requestID string, issues []ValidationIssue) {
	api.FailWithDetails(
		w,
		http.StatusBadRequest,
		"validation_error",
		"payload validation failed",
		map[string]any{"fields": issues},
		requestID,
	)
}
