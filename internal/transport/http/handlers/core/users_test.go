package corehandler

import (
	"testing"

	"hrm/internal/domain/auth"
)

func TestNormalizeRoleName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "employee", want: auth.RoleEmployee},
		{in: "Manager", want: auth.RoleManager},
		{in: "hrmanager", want: auth.RoleHRManager},
		{in: "hr_manager", want: auth.RoleHRManager},
		{in: "HR", want: auth.RoleHR},
		{in: "admin", want: auth.RoleAdmin},
		{in: "system_admin", want: auth.RoleSystemAdmin},
		{in: "unknown", want: ""},
	}
	for _, tc := range cases {
		if got := normalizeRoleName(tc.in); got != tc.want {
			t.Fatalf("normalizeRoleName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCanCreateRoleMatrix(t *testing.T) {
	tests := []struct {
		actor  string
		target string
		allow  bool
	}{
		{actor: auth.RoleSystemAdmin, target: auth.RoleHR, allow: true},
		{actor: auth.RoleSystemAdmin, target: auth.RoleHRManager, allow: true},
		{actor: auth.RoleSystemAdmin, target: auth.RoleManager, allow: true},
		{actor: auth.RoleSystemAdmin, target: auth.RoleAdmin, allow: true},
		{actor: auth.RoleSystemAdmin, target: auth.RoleEmployee, allow: false},
		{actor: auth.RoleAdmin, target: auth.RoleHRManager, allow: true},
		{actor: auth.RoleAdmin, target: auth.RoleHR, allow: true},
		{actor: auth.RoleAdmin, target: auth.RoleManager, allow: true},
		{actor: auth.RoleAdmin, target: auth.RoleEmployee, allow: false},
		{actor: auth.RoleAdmin, target: auth.RoleAdmin, allow: false},
		{actor: auth.RoleHRManager, target: auth.RoleHR, allow: true},
		{actor: auth.RoleHRManager, target: auth.RoleEmployee, allow: true},
		{actor: auth.RoleHRManager, target: auth.RoleManager, allow: false},
		{actor: auth.RoleHR, target: auth.RoleEmployee, allow: true},
		{actor: auth.RoleHR, target: auth.RoleHR, allow: false},
		{actor: auth.RoleManager, target: auth.RoleEmployee, allow: false},
	}

	for _, tc := range tests {
		if got := canCreateRole(tc.actor, tc.target); got != tc.allow {
			t.Fatalf("canCreateRole(%q,%q) = %v, want %v", tc.actor, tc.target, got, tc.allow)
		}
	}
}

func TestCanManageRoleMatrix(t *testing.T) {
	tests := []struct {
		actor  string
		target string
		allow  bool
	}{
		{actor: auth.RoleSystemAdmin, target: auth.RoleHR, allow: true},
		{actor: auth.RoleSystemAdmin, target: auth.RoleHRManager, allow: true},
		{actor: auth.RoleSystemAdmin, target: auth.RoleManager, allow: true},
		{actor: auth.RoleSystemAdmin, target: auth.RoleAdmin, allow: true},
		{actor: auth.RoleSystemAdmin, target: auth.RoleEmployee, allow: true},
		{actor: auth.RoleSystemAdmin, target: auth.RoleSystemAdmin, allow: false},
		{actor: auth.RoleAdmin, target: auth.RoleHR, allow: true},
		{actor: auth.RoleAdmin, target: auth.RoleHRManager, allow: true},
		{actor: auth.RoleAdmin, target: auth.RoleManager, allow: true},
		{actor: auth.RoleAdmin, target: auth.RoleEmployee, allow: true},
		{actor: auth.RoleAdmin, target: auth.RoleAdmin, allow: false},
		{actor: auth.RoleAdmin, target: auth.RoleSystemAdmin, allow: false},
		{actor: auth.RoleHRManager, target: auth.RoleHR, allow: true},
		{actor: auth.RoleHRManager, target: auth.RoleEmployee, allow: true},
		{actor: auth.RoleHRManager, target: auth.RoleManager, allow: false},
		{actor: auth.RoleHR, target: auth.RoleEmployee, allow: true},
		{actor: auth.RoleHR, target: auth.RoleHR, allow: false},
	}
	for _, tc := range tests {
		if got := canManageRole(tc.actor, tc.target); got != tc.allow {
			t.Fatalf("canManageRole(%q,%q) = %v, want %v", tc.actor, tc.target, got, tc.allow)
		}
	}
}
