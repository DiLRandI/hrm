package auth

type UserContext struct {
	UserID    string
	TenantID  string
	RoleID    string
	RoleName  string
	SessionID string
}
