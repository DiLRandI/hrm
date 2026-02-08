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

func TestCalculateRequestDays(t *testing.T) {
	tests := []struct {
		name      string
		start     time.Time
		end       time.Time
		startHalf bool
		endHalf   bool
		wantDays  float64
		wantErr   bool
	}{
		{
			name:     "single full day",
			start:    time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			wantDays: 1,
		},
		{
			name:      "single half day start",
			start:     time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			end:       time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			startHalf: true,
			wantDays:  0.5,
		},
		{
			name:     "single half day end",
			start:    time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			endHalf:  true,
			wantDays: 0.5,
		},
		{
			name:      "single day both halves invalid",
			start:     time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			end:       time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			startHalf: true,
			endHalf:   true,
			wantErr:   true,
		},
		{
			name:      "multi day both half boundaries",
			start:     time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			end:       time.Date(2025, 1, 12, 0, 0, 0, 0, time.UTC),
			startHalf: true,
			endHalf:   true,
			wantDays:  2,
		},
		{
			name:    "end before start invalid",
			start:   time.Date(2025, 1, 12, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := CalculateRequestDays(tc.start, tc.end, tc.startHalf, tc.endHalf)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantDays {
				t.Fatalf("expected %v days, got %v", tc.wantDays, got)
			}
		})
	}
}
