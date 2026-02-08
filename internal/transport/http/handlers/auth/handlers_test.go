package authhandler

import (
	"strings"
	"testing"
	"time"
)

func TestValidateResetPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "Stronger123",
		},
		{
			name:     "too short",
			password: "S1hort",
			wantErr:  true,
		},
		{
			name:     "missing uppercase",
			password: "longpassword1",
			wantErr:  true,
		},
		{
			name:     "missing lowercase",
			password: "LONGPASSWORD1",
			wantErr:  true,
		},
		{
			name:     "missing number",
			password: "LongPassword",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateResetPassword(tc.password)
			if tc.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestBuildResetLink(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		token     string
		wantParts []string
	}{
		{
			name:      "empty base url uses default",
			baseURL:   "",
			token:     "abc",
			wantParts: []string{"http://localhost:8080/reset", "token=abc"},
		},
		{
			name:      "custom host",
			baseURL:   "https://hr.example.com",
			token:     "token123",
			wantParts: []string{"https://hr.example.com/reset", "token=token123"},
		},
		{
			name:      "custom path",
			baseURL:   "https://hr.example.com/app",
			token:     "xyz",
			wantParts: []string{"https://hr.example.com/app/reset", "token=xyz"},
		},
		{
			name:      "invalid base url falls back",
			baseURL:   "not a url",
			token:     "abc",
			wantParts: []string{"http://localhost:8080/reset", "token=abc"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := buildResetLink(tc.baseURL, tc.token)
			for _, part := range tc.wantParts {
				if !strings.Contains(got, part) {
					t.Fatalf("expected reset link %q to contain %q", got, part)
				}
			}
		})
	}
}

func TestBuildResetEmailMessage(t *testing.T) {
	link := "https://hr.example.com/reset?token=abc"
	msg := buildResetEmailMessage(link, 2*time.Hour)
	if !strings.Contains(msg, link) {
		t.Fatalf("expected email message to include reset link, got %q", msg)
	}
	if !strings.Contains(msg, "expires in 2 hour(s)") {
		t.Fatalf("expected email message to include ttl, got %q", msg)
	}
}
