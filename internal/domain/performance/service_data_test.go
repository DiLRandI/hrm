package performance

import "testing"

func TestBuildPerformanceSummaryWithRatings(t *testing.T) {
	summary := buildPerformanceSummary(10, 6, 8, 4, []float64{3.2, 3.7, 4.1, 4.9})
	if summary.GoalsTotal != 10 || summary.GoalsCompleted != 6 {
		t.Fatalf("unexpected goals summary: %+v", summary)
	}
	if summary.ReviewTasksTotal != 8 || summary.ReviewTasksCompleted != 4 {
		t.Fatalf("unexpected task summary: %+v", summary)
	}
	if summary.ReviewCompletionRate != 0.5 {
		t.Fatalf("expected completion rate 0.5, got %v", summary.ReviewCompletionRate)
	}
	if summary.RatingDistribution["3"] != 1 {
		t.Fatalf("expected one rating rounded to 3, got %d", summary.RatingDistribution["3"])
	}
	if summary.RatingDistribution["4"] != 2 {
		t.Fatalf("expected two ratings rounded to 4, got %d", summary.RatingDistribution["4"])
	}
	if summary.RatingDistribution["5"] != 1 {
		t.Fatalf("expected one rating rounded to 5, got %d", summary.RatingDistribution["5"])
	}
}

func TestBuildPerformanceSummaryHandlesZeroTasks(t *testing.T) {
	summary := buildPerformanceSummary(2, 1, 0, 0, nil)
	if summary.ReviewCompletionRate != 0 {
		t.Fatalf("expected zero completion rate, got %v", summary.ReviewCompletionRate)
	}
	if len(summary.RatingDistribution) != 0 {
		t.Fatalf("expected empty rating distribution, got %+v", summary.RatingDistribution)
	}
}
