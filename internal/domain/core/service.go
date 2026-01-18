package core

import "context"

type Service struct {
	Store *Store
}

func NewService(store *Store) *Service {
	return &Service{Store: store}
}

func (s *Service) MFAEnabled(ctx context.Context, userID string) (bool, error) {
	return s.Store.MFAEnabled(ctx, userID)
}

func (s *Service) InsertAccessLog(ctx context.Context, tenantID, actorID, employeeID, requestID string, fields []string) error {
	return s.Store.InsertAccessLog(ctx, tenantID, actorID, employeeID, requestID, fields)
}

func (s *Service) CreateManagerRelation(ctx context.Context, employeeID, managerID string) error {
	return s.Store.CreateManagerRelation(ctx, employeeID, managerID)
}

func (s *Service) CloseManagerRelations(ctx context.Context, employeeID string) error {
	return s.Store.CloseManagerRelations(ctx, employeeID)
}

func (s *Service) DepartmentCount(ctx context.Context, tenantID string) (int, error) {
	return s.Store.DepartmentCount(ctx, tenantID)
}

func (s *Service) ListDepartments(ctx context.Context, tenantID string, limit, offset int) ([]Department, error) {
	return s.Store.ListDepartments(ctx, tenantID, limit, offset)
}

func (s *Service) CreateDepartment(ctx context.Context, tenantID string, dep Department) (string, error) {
	return s.Store.CreateDepartment(ctx, tenantID, dep)
}

func (s *Service) UpdateDepartment(ctx context.Context, tenantID, departmentID string, dep Department) (bool, error) {
	return s.Store.UpdateDepartment(ctx, tenantID, departmentID, dep)
}

func (s *Service) DepartmentHasEmployees(ctx context.Context, tenantID, departmentID string) (bool, error) {
	return s.Store.DepartmentHasEmployees(ctx, tenantID, departmentID)
}

func (s *Service) DeleteDepartment(ctx context.Context, tenantID, departmentID string) error {
	return s.Store.DeleteDepartment(ctx, tenantID, departmentID)
}

func (s *Service) OrgChartNodes(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	return s.Store.OrgChartNodes(ctx, tenantID, employeeID)
}

func (s *Service) ManagerHistory(ctx context.Context, employeeID string) ([]map[string]any, error) {
	return s.Store.ManagerHistory(ctx, employeeID)
}

func (s *Service) ListPermissions(ctx context.Context) ([]map[string]string, error) {
	return s.Store.ListPermissions(ctx)
}

func (s *Service) ListRoles(ctx context.Context, tenantID string) ([]map[string]any, error) {
	return s.Store.ListRoles(ctx, tenantID)
}

func (s *Service) RoleTenant(ctx context.Context, roleID string) (string, error) {
	return s.Store.RoleTenant(ctx, roleID)
}

func (s *Service) UpdateRolePermissions(ctx context.Context, roleID string, permissions []string) error {
	return s.Store.UpdateRolePermissions(ctx, roleID, permissions)
}

func (s *Service) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	return s.Store.EmployeeIDByUserID(ctx, tenantID, userID)
}

func (s *Service) ManagerIDByEmployeeID(ctx context.Context, tenantID, employeeID string) (string, error) {
	return s.Store.ManagerIDByEmployeeID(ctx, tenantID, employeeID)
}

func (s *Service) UserIDByEmployeeID(ctx context.Context, tenantID, employeeID string) (string, error) {
	return s.Store.UserIDByEmployeeID(ctx, tenantID, employeeID)
}
