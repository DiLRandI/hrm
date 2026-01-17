package auth

import "testing"

func TestRolePermissionsSubset(t *testing.T) {
	allowed := map[string]struct{}{}
	for _, perm := range DefaultPermissions {
		allowed[perm] = struct{}{}
	}

	for role, perms := range RolePermissions {
		if len(perms) == 0 {
			t.Fatalf("role %s has no permissions", role)
		}
		for _, perm := range perms {
			if _, ok := allowed[perm]; !ok {
				t.Fatalf("role %s has unknown permission %s", role, perm)
			}
		}
	}
}

func TestDefaultPermissionsUnique(t *testing.T) {
	seen := map[string]struct{}{}
	for _, perm := range DefaultPermissions {
		if _, ok := seen[perm]; ok {
			t.Fatalf("duplicate permission %s", perm)
		}
		seen[perm] = struct{}{}
	}
}
