package leave

import (
	"testing"
	"time"
)

func TestCalculateDays(t *testing.T) {
	start := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)

	days, err := CalculateDays(start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if days != 1 {
		t.Fatalf("expected 1 day, got %v", days)
	}

	end = time.Date(2025, 1, 12, 0, 0, 0, 0, time.UTC)
	days, err = CalculateDays(start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if days != 3 {
		t.Fatalf("expected 3 days, got %v", days)
	}
}

func TestCalculateDaysInvalid(t *testing.T) {
	start := time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 2, 9, 0, 0, 0, 0, time.UTC)

	_, err := CalculateDays(start, end)
	if err == nil {
		t.Fatal("expected error for invalid range")
	}
}
