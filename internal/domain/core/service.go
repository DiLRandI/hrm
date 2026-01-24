package core

import "context"

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) MFAEnabled(ctx context.Context, userID string) (bool, error) {
	return s.store.MFAEnabled(ctx, userID)
}

func (s *Service) InsertAccessLog(ctx context.Context, tenantID, actorID, employeeID, requestID string, fields []string) error {
	return s.store.InsertAccessLog(ctx, tenantID, actorID, employeeID, requestID, fields)
}

func (s *Service) CreateManagerRelation(ctx context.Context, employeeID, managerID string) error {
	return s.store.CreateManagerRelation(ctx, employeeID, managerID)
}

func (s *Service) CloseManagerRelations(ctx context.Context, employeeID string) error {
	return s.store.CloseManagerRelations(ctx, employeeID)
}

func (s *Service) DepartmentCount(ctx context.Context, tenantID string) (int, error) {
	return s.store.DepartmentCount(ctx, tenantID)
}

func (s *Service) ListDepartments(ctx context.Context, tenantID string, limit, offset int) ([]Department, error) {
	return s.store.ListDepartments(ctx, tenantID, limit, offset)
}

func (s *Service) CreateDepartment(ctx context.Context, tenantID string, dep Department) (string, error) {
	return s.store.CreateDepartment(ctx, tenantID, dep)
}

func (s *Service) UpdateDepartment(ctx context.Context, tenantID, departmentID string, dep Department) (bool, error) {
	return s.store.UpdateDepartment(ctx, tenantID, departmentID, dep)
}

func (s *Service) DepartmentHasEmployees(ctx context.Context, tenantID, departmentID string) (bool, error) {
	return s.store.DepartmentHasEmployees(ctx, tenantID, departmentID)
}

func (s *Service) DeleteDepartment(ctx context.Context, tenantID, departmentID string) error {
	return s.store.DeleteDepartment(ctx, tenantID, departmentID)
}

func (s *Service) OrgChartNodes(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.store.OrgChartNodes(ctx, tenantID, employeeID)
}

func (s *Service) ManagerHistory(ctx context.Context, employeeID string) ([]map[string]any, error) {
	return s.store.ManagerHistory(ctx, employeeID)
}

func (s *Service) ListPermissions(ctx context.Context) ([]map[string]string, error) {
	return s.store.ListPermissions(ctx)
}

func (s *Service) ListRoles(ctx context.Context, tenantID string) ([]map[string]any, error) {
	return s.store.ListRoles(ctx, tenantID)
}

func (s *Service) RoleTenant(ctx context.Context, roleID string) (string, error) {
	return s.store.RoleTenant(ctx, roleID)
}

func (s *Service) UpdateRolePermissions(ctx context.Context, roleID string, permissions []string) error {
	return s.store.UpdateRolePermissions(ctx, roleID, permissions)
}

func (s *Service) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	return s.store.EmployeeIDByUserID(ctx, tenantID, userID)
}

func (s *Service) ManagerIDByEmployeeID(ctx context.Context, tenantID, employeeID string) (string, error) {
	return s.store.ManagerIDByEmployeeID(ctx, tenantID, employeeID)
}

func (s *Service) UserIDByEmployeeID(ctx context.Context, tenantID, employeeID string) (string, error) {
	return s.store.UserIDByEmployeeID(ctx, tenantID, employeeID)
}

func (s *Service) HasPermission(ctx context.Context, roleID, permission string) (bool, error) {
	return s.store.HasPermission(ctx, roleID, permission)
}

func (s *Service) UserExists(ctx context.Context, tenantID, userID string) (bool, error) {
	return s.store.UserExists(ctx, tenantID, userID)
}

func (s *Service) GetEmployee(ctx context.Context, tenantID, employeeID string) (*Employee, error) {
	return s.store.GetEmployee(ctx, tenantID, employeeID)
}

func (s *Service) GetEmployeeByUserID(ctx context.Context, tenantID, userID string) (*Employee, error) {
	return s.store.GetEmployeeByUserID(ctx, tenantID, userID)
}

func (s *Service) ListEmployees(ctx context.Context, tenantID string) ([]Employee, error) {
	return s.store.ListEmployees(ctx, tenantID)
}

func (s *Service) CreateEmployee(ctx context.Context, tenantID string, emp Employee) (string, error) {
	return s.store.CreateEmployee(ctx, tenantID, emp)
}

func (s *Service) CreateEmployeeWithUser(ctx context.Context, tenantID string, emp Employee, password string) (string, string, error) {
	return s.store.CreateEmployeeWithUser(ctx, tenantID, emp, password)
}

func (s *Service) UpdateEmployee(ctx context.Context, tenantID, employeeID string, emp Employee) error {
	return s.store.UpdateEmployee(ctx, tenantID, employeeID, emp)
}

func (s *Service) ListEmergencyContacts(ctx context.Context, tenantID, employeeID string) ([]EmergencyContact, error) {
	return s.store.ListEmergencyContacts(ctx, tenantID, employeeID)
}

func (s *Service) ReplaceEmergencyContacts(ctx context.Context, tenantID, employeeID string, contacts []EmergencyContact) error {
	return s.store.ReplaceEmergencyContacts(ctx, tenantID, employeeID, contacts)
}

func (s *Service) IsManagerOf(ctx context.Context, tenantID, managerEmployeeID, employeeID string) (bool, error) {
	return s.store.IsManagerOf(ctx, tenantID, managerEmployeeID, employeeID)
}
