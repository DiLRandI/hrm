package core

import "testing"

func TestNullIfEmpty(t *testing.T) {
	if value := nullIfEmpty(""); value != nil {
		t.Fatal("expected nil for empty string")
	}
	if value := nullIfEmpty("value"); value == nil {
		t.Fatal("expected value for non-empty string")
	}
}
