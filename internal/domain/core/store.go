package core

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"

	"hrm/internal/domain/auth"
	cryptoutil "hrm/internal/platform/crypto"
	"hrm/internal/platform/querier"
)

type Store struct {
	DB     querier.Querier
	Crypto *cryptoutil.Service
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func NewStore(db querier.Querier, crypto *cryptoutil.Service) *Store {
	return &Store{DB: db, Crypto: crypto}
}

func (s *Store) HasPermission(ctx context.Context, roleID, permission string) (bool, error) {
	var count int
	err := s.DB.QueryRow(ctx, `
    SELECT COUNT(1)
    FROM role_permissions rp
    JOIN permissions p ON rp.permission_id = p.id
    WHERE rp.role_id = $1 AND p.key = $2
  `, roleID, permission).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) UserExists(ctx context.Context, tenantID, userID string) (bool, error) {
	var count int
	err := s.DB.QueryRow(ctx, `
    SELECT COUNT(1)
    FROM users
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) GetEmployee(ctx context.Context, tenantID, employeeID string) (*Employee, error) {
	row := s.DB.QueryRow(ctx, `
    SELECT id,
           COALESCE(user_id::text, ''),
           COALESCE(employee_number, ''),
           first_name, last_name, email,
           COALESCE(phone, ''),
           date_of_birth,
           COALESCE(address, ''),
           COALESCE(national_id, ''),
           national_id_enc,
           COALESCE(bank_account, ''),
           bank_account_enc,
           salary,
           salary_enc,
           currency,
           COALESCE(employment_type, ''),
           COALESCE(department_id::text, ''),
           COALESCE(manager_id::text, ''),
           COALESCE(pay_group_id::text, ''),
           start_date, end_date, status, created_at, updated_at
    FROM employees
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, employeeID)

	var emp Employee
	var nationalEnc, bankEnc, salaryEnc []byte
	var nationalPlain, bankPlain string
	var salaryPlain *float64
	err := row.Scan(
		&emp.ID, &emp.UserID, &emp.EmployeeNumber, &emp.FirstName, &emp.LastName, &emp.Email, &emp.Phone,
		&emp.DateOfBirth, &emp.Address, &nationalPlain, &nationalEnc, &bankPlain, &bankEnc, &salaryPlain, &salaryEnc,
		&emp.Currency, &emp.EmploymentType, &emp.DepartmentID, &emp.ManagerID, &emp.PayGroupID,
		&emp.StartDate, &emp.EndDate, &emp.Status,
		&emp.CreatedAt, &emp.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	emp.NationalID = decryptStringFallback(s.Crypto, nationalEnc, nationalPlain)
	emp.BankAccount = decryptStringFallback(s.Crypto, bankEnc, bankPlain)
	emp.Salary = decryptFloatFallback(s.Crypto, salaryEnc, salaryPlain)
	return &emp, nil
}

func (s *Store) GetEmployeeByUserID(ctx context.Context, tenantID, userID string) (*Employee, error) {
	row := s.DB.QueryRow(ctx, `
    SELECT id,
           COALESCE(user_id::text, ''),
           COALESCE(employee_number, ''),
           first_name, last_name, email,
           COALESCE(phone, ''),
           date_of_birth,
           COALESCE(address, ''),
           COALESCE(national_id, ''),
           national_id_enc,
           COALESCE(bank_account, ''),
           bank_account_enc,
           salary,
           salary_enc,
           currency,
           COALESCE(employment_type, ''),
           COALESCE(department_id::text, ''),
           COALESCE(manager_id::text, ''),
           COALESCE(pay_group_id::text, ''),
           start_date, end_date, status, created_at, updated_at
    FROM employees
    WHERE tenant_id = $1 AND user_id = $2
  `, tenantID, userID)

	var emp Employee
	var nationalEnc, bankEnc, salaryEnc []byte
	var nationalPlain, bankPlain string
	var salaryPlain *float64
	if err := row.Scan(
		&emp.ID, &emp.UserID, &emp.EmployeeNumber, &emp.FirstName, &emp.LastName, &emp.Email, &emp.Phone,
		&emp.DateOfBirth, &emp.Address, &nationalPlain, &nationalEnc, &bankPlain, &bankEnc, &salaryPlain, &salaryEnc,
		&emp.Currency, &emp.EmploymentType, &emp.DepartmentID, &emp.ManagerID, &emp.PayGroupID,
		&emp.StartDate, &emp.EndDate, &emp.Status, &emp.CreatedAt, &emp.UpdatedAt,
	); err != nil {
		return nil, err
	}
	emp.NationalID = decryptStringFallback(s.Crypto, nationalEnc, nationalPlain)
	emp.BankAccount = decryptStringFallback(s.Crypto, bankEnc, bankPlain)
	emp.Salary = decryptFloatFallback(s.Crypto, salaryEnc, salaryPlain)
	return &emp, nil
}

func (s *Store) IsManagerOf(ctx context.Context, tenantID, managerEmployeeID, employeeID string) (bool, error) {
	var count int
	err := s.DB.QueryRow(ctx, `
    SELECT COUNT(1)
    FROM employees
    WHERE tenant_id = $1 AND id = $2 AND manager_id = $3
  `, tenantID, employeeID, managerEmployeeID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) ListEmployees(ctx context.Context, tenantID string) ([]Employee, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id,
           COALESCE(user_id::text, ''),
           COALESCE(employee_number, ''),
           first_name, last_name, email,
           COALESCE(phone, ''),
           date_of_birth,
           COALESCE(address, ''),
           COALESCE(national_id, ''),
           national_id_enc,
           COALESCE(bank_account, ''),
           bank_account_enc,
           salary,
           salary_enc,
           currency,
           COALESCE(employment_type, ''),
           COALESCE(department_id::text, ''),
           COALESCE(manager_id::text, ''),
           COALESCE(pay_group_id::text, ''),
           start_date, end_date, status, created_at, updated_at
    FROM employees
    WHERE tenant_id = $1
    ORDER BY last_name, first_name
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Employee
	for rows.Next() {
		var emp Employee
		var nationalEnc, bankEnc, salaryEnc []byte
		var nationalPlain, bankPlain string
		var salaryPlain *float64
		if err := rows.Scan(
			&emp.ID, &emp.UserID, &emp.EmployeeNumber, &emp.FirstName, &emp.LastName, &emp.Email, &emp.Phone,
			&emp.DateOfBirth, &emp.Address, &nationalPlain, &nationalEnc, &bankPlain, &bankEnc, &salaryPlain, &salaryEnc,
			&emp.Currency, &emp.EmploymentType, &emp.DepartmentID, &emp.ManagerID, &emp.PayGroupID,
			&emp.StartDate, &emp.EndDate, &emp.Status,
			&emp.CreatedAt, &emp.UpdatedAt,
		); err != nil {
			return nil, err
		}
		emp.NationalID = decryptStringFallback(s.Crypto, nationalEnc, nationalPlain)
		emp.BankAccount = decryptStringFallback(s.Crypto, bankEnc, bankPlain)
		emp.Salary = decryptFloatFallback(s.Crypto, salaryEnc, salaryPlain)
		out = append(out, emp)
	}
	return out, nil
}

func (s *Store) CreateEmployee(ctx context.Context, tenantID string, emp Employee) (string, error) {
	return s.createEmployee(ctx, s.DB, tenantID, emp, emp.UserID)
}

func (s *Store) CreateEmployeeWithUser(ctx context.Context, tenantID string, emp Employee, password string) (string, string, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)

	var roleID string
	if err := tx.QueryRow(ctx, "SELECT id FROM roles WHERE tenant_id = $1 AND name = $2", tenantID, auth.RoleEmployee).Scan(&roleID); err != nil {
		return "", "", err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return "", "", err
	}

	var userID string
	if err := tx.QueryRow(ctx, `
    INSERT INTO users (tenant_id, email, password_hash, role_id)
    VALUES ($1, $2, $3, $4)
    RETURNING id
  `, tenantID, emp.Email, hash, roleID).Scan(&userID); err != nil {
		return "", "", err
	}

	employeeID, err := s.createEmployee(ctx, tx, tenantID, emp, userID)
	if err != nil {
		return "", "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}

	return employeeID, userID, nil
}

func (s *Store) createEmployee(ctx context.Context, q rowQuerier, tenantID string, emp Employee, userID string) (string, error) {
	nationalEnc, bankEnc, salaryEnc, encErr := encryptEmployeeSensitive(s.Crypto, emp)
	if encErr != nil {
		return "", encErr
	}
	var nationalPlain, bankPlain any = emp.NationalID, emp.BankAccount
	var salaryPlain any = emp.Salary
	if s.Crypto != nil && s.Crypto.Configured() {
		nationalPlain = nil
		bankPlain = nil
		salaryPlain = nil
	}
	if userID == "" {
		userID = emp.UserID
	}
	var id string
	err := q.QueryRow(ctx, `
    INSERT INTO employees (tenant_id, user_id, employee_number, first_name, last_name, email, phone, date_of_birth,
      address, national_id, national_id_enc, bank_account, bank_account_enc, salary, salary_enc, currency,
      employment_type, department_id, manager_id, pay_group_id, start_date, end_date, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
    RETURNING id
  `,
		tenantID, nullIfEmpty(userID), nullIfEmpty(emp.EmployeeNumber), emp.FirstName, emp.LastName, emp.Email, emp.Phone,
		emp.DateOfBirth, emp.Address, nationalPlain, nationalEnc, bankPlain, bankEnc, salaryPlain, salaryEnc,
		emp.Currency, emp.EmploymentType, nullIfEmpty(emp.DepartmentID), nullIfEmpty(emp.ManagerID), nullIfEmpty(emp.PayGroupID),
		emp.StartDate, emp.EndDate, emp.Status,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) UpdateEmployee(ctx context.Context, tenantID, employeeID string, emp Employee) error {
	nationalEnc, bankEnc, salaryEnc, encErr := encryptEmployeeSensitive(s.Crypto, emp)
	if encErr != nil {
		return encErr
	}
	var nationalPlain, bankPlain any = emp.NationalID, emp.BankAccount
	var salaryPlain any = emp.Salary
	if s.Crypto != nil && s.Crypto.Configured() {
		nationalPlain = nil
		bankPlain = nil
		salaryPlain = nil
	}
	cmd, err := s.DB.Exec(ctx, `
    UPDATE employees
    SET employee_number = $1,
        first_name = $2,
        last_name = $3,
        email = $4,
        phone = $5,
        date_of_birth = $6,
        address = $7,
        national_id = $8,
        national_id_enc = $9,
        bank_account = $10,
        bank_account_enc = $11,
        salary = $12,
        salary_enc = $13,
        currency = $14,
        employment_type = $15,
        department_id = $16,
        manager_id = $17,
        pay_group_id = $18,
        start_date = $19,
        end_date = $20,
        status = $21,
        updated_at = now()
    WHERE tenant_id = $22 AND id = $23
  `,
		emp.EmployeeNumber, emp.FirstName, emp.LastName, emp.Email, emp.Phone, emp.DateOfBirth, emp.Address,
		nationalPlain, nationalEnc, bankPlain, bankEnc, salaryPlain, salaryEnc, emp.Currency, emp.EmploymentType,
		nullIfEmpty(emp.DepartmentID), nullIfEmpty(emp.ManagerID), nullIfEmpty(emp.PayGroupID),
		emp.StartDate, emp.EndDate, emp.Status, tenantID, employeeID,
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("employee not found")
	}
	return nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s *Store) MFAEnabled(ctx context.Context, userID string) (bool, error) {
	var enabled bool
	if err := s.DB.QueryRow(ctx, "SELECT mfa_enabled FROM users WHERE id = $1", userID).Scan(&enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

func (s *Store) InsertAccessLog(ctx context.Context, tenantID, actorID, employeeID, requestID string, fields []string) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO access_logs (tenant_id, actor_user_id, employee_id, fields, request_id)
    VALUES ($1,$2,$3,$4,$5)
  `, tenantID, actorID, employeeID, fields, requestID)
	return err
}

func (s *Store) CreateManagerRelation(ctx context.Context, employeeID, managerID string) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO manager_relations (employee_id, manager_id, start_date)
    VALUES ($1, $2, CURRENT_DATE)
  `, employeeID, managerID)
	return err
}

func (s *Store) CloseManagerRelations(ctx context.Context, employeeID string) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE manager_relations
    SET end_date = CURRENT_DATE
    WHERE employee_id = $1 AND end_date IS NULL
  `, employeeID)
	return err
}

func (s *Store) DepartmentCount(ctx context.Context, tenantID string) (int, error) {
	var total int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM departments WHERE tenant_id = $1", tenantID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) ListDepartments(ctx context.Context, tenantID string, limit, offset int) ([]Department, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, name, parent_id, manager_id, created_at
    FROM departments
    WHERE tenant_id = $1
    ORDER BY name
    LIMIT $2 OFFSET $3
  `, tenantID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var departments []Department
	for rows.Next() {
		var dep Department
		if err := rows.Scan(&dep.ID, &dep.Name, &dep.ParentID, &dep.ManagerID, &dep.CreatedAt); err != nil {
			return nil, err
		}
		departments = append(departments, dep)
	}
	return departments, nil
}

func (s *Store) CreateDepartment(ctx context.Context, tenantID string, dep Department) (string, error) {
	var id string
	err := s.DB.QueryRow(ctx, `
    INSERT INTO departments (tenant_id, name, parent_id, manager_id)
    VALUES ($1, $2, $3, $4)
    RETURNING id
  `, tenantID, dep.Name, nullIfEmpty(dep.ParentID), nullIfEmpty(dep.ManagerID)).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) UpdateDepartment(ctx context.Context, tenantID, departmentID string, dep Department) (bool, error) {
	cmd, err := s.DB.Exec(ctx, `
    UPDATE departments
    SET name = $1, parent_id = $2, manager_id = $3
    WHERE tenant_id = $4 AND id = $5
  `, dep.Name, nullIfEmpty(dep.ParentID), nullIfEmpty(dep.ManagerID), tenantID, departmentID)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

func (s *Store) DepartmentHasEmployees(ctx context.Context, tenantID, departmentID string) (bool, error) {
	var count int
	if err := s.DB.QueryRow(ctx, `
    SELECT COUNT(1) FROM employees WHERE tenant_id = $1 AND department_id = $2
  `, tenantID, departmentID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) DeleteDepartment(ctx context.Context, tenantID, departmentID string) error {
	_, err := s.DB.Exec(ctx, `
    DELETE FROM departments WHERE tenant_id = $1 AND id = $2
  `, tenantID, departmentID)
	return err
}

func (s *Store) OrgChartNodes(ctx context.Context, tenantID, employeeID string) ([]map[string]any, error) {
	query := `
    SELECT id, first_name, last_name, COALESCE(manager_id::text,''), COALESCE(department_id::text,'')
    FROM employees
    WHERE tenant_id = $1`
	args := []any{tenantID}
	if employeeID != "" {
		query += " AND (id = $2 OR manager_id = $2)"
		args = append(args, employeeID)
	}
	query += " ORDER BY last_name, first_name"

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []map[string]any
	for rows.Next() {
		var id, first, last, managerID, departmentID string
		if err := rows.Scan(&id, &first, &last, &managerID, &departmentID); err != nil {
			return nil, err
		}
		nodes = append(nodes, map[string]any{
			"id":           id,
			"name":         first + " " + last,
			"managerId":    managerID,
			"departmentId": departmentID,
		})
	}
	return nodes, nil
}

func (s *Store) ManagerHistory(ctx context.Context, employeeID string) ([]map[string]any, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT mr.manager_id, COALESCE(m.first_name,''), COALESCE(m.last_name,''), mr.start_date, mr.end_date
    FROM manager_relations mr
    LEFT JOIN employees m ON mr.manager_id = m.id
    WHERE mr.employee_id = $1
    ORDER BY mr.start_date DESC
  `, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var history []map[string]any
	for rows.Next() {
		var managerID, first, last string
		var startDate, endDate any
		if err := rows.Scan(&managerID, &first, &last, &startDate, &endDate); err != nil {
			return nil, err
		}
		history = append(history, map[string]any{
			"managerId": managerID,
			"name":      first + " " + last,
			"startDate": startDate,
			"endDate":   endDate,
		})
	}
	return history, nil
}

func (s *Store) ListPermissions(ctx context.Context) ([]map[string]string, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT key, COALESCE(description,'')
    FROM permissions
    ORDER BY key
  `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]string
	for rows.Next() {
		var key, desc string
		if err := rows.Scan(&key, &desc); err != nil {
			return nil, err
		}
		out = append(out, map[string]string{"key": key, "description": desc})
	}
	return out, nil
}

func (s *Store) ListRoles(ctx context.Context, tenantID string) ([]map[string]any, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT r.id,
           r.name,
           COALESCE(array_agg(p.key) FILTER (WHERE p.key IS NOT NULL), '{}') AS permissions
    FROM roles r
    LEFT JOIN role_permissions rp ON rp.role_id = r.id
    LEFT JOIN permissions p ON rp.permission_id = p.id
    WHERE r.tenant_id = $1
    GROUP BY r.id
    ORDER BY r.name
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []map[string]any
	for rows.Next() {
		var id, name string
		var perms []string
		if err := rows.Scan(&id, &name, &perms); err != nil {
			return nil, err
		}
		roles = append(roles, map[string]any{"id": id, "name": name, "permissions": perms})
	}
	return roles, nil
}

func (s *Store) RoleTenant(ctx context.Context, roleID string) (string, error) {
	var tenantID string
	if err := s.DB.QueryRow(ctx, "SELECT tenant_id::text FROM roles WHERE id = $1", roleID).Scan(&tenantID); err != nil {
		return "", err
	}
	return tenantID, nil
}

func (s *Store) UpdateRolePermissions(ctx context.Context, roleID string, permissions []string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "DELETE FROM role_permissions WHERE role_id = $1", roleID); err != nil {
		return err
	}

	if len(permissions) > 0 {
		rows, err := tx.Query(ctx, `
      SELECT id, key FROM permissions WHERE key = ANY($1)
    `, permissions)
		if err != nil {
			return err
		}
		var permissionIDs []string
		for rows.Next() {
			var id, key string
			if err := rows.Scan(&id, &key); err != nil {
				rows.Close()
				return err
			}
			permissionIDs = append(permissionIDs, id)
		}
		rows.Close()
		for _, permID := range permissionIDs {
			if _, err := tx.Exec(ctx, "INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2)", roleID, permID); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	var employeeID string
	if err := s.DB.QueryRow(ctx, "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", tenantID, userID).Scan(&employeeID); err != nil {
		return "", err
	}
	return employeeID, nil
}

func (s *Store) ManagerIDByEmployeeID(ctx context.Context, tenantID, employeeID string) (string, error) {
	var managerID string
	if err := s.DB.QueryRow(ctx, "SELECT manager_id FROM employees WHERE tenant_id = $1 AND id = $2", tenantID, employeeID).Scan(&managerID); err != nil {
		return "", err
	}
	return managerID, nil
}

func (s *Store) UserIDByEmployeeID(ctx context.Context, tenantID, employeeID string) (string, error) {
	var userID string
	if err := s.DB.QueryRow(ctx, "SELECT user_id FROM employees WHERE tenant_id = $1 AND id = $2", tenantID, employeeID).Scan(&userID); err != nil {
		return "", err
	}
	return userID, nil
}

func encryptEmployeeSensitive(crypto *cryptoutil.Service, emp Employee) ([]byte, []byte, []byte, error) {
	if crypto == nil || !crypto.Configured() {
		return nil, nil, nil, nil
	}
	var nationalEnc []byte
	if emp.NationalID != "" {
		enc, err := crypto.EncryptString(emp.NationalID)
		if err != nil {
			return nil, nil, nil, err
		}
		nationalEnc = enc
	}
	var bankEnc []byte
	if emp.BankAccount != "" {
		enc, err := crypto.EncryptString(emp.BankAccount)
		if err != nil {
			return nil, nil, nil, err
		}
		bankEnc = enc
	}
	var salaryEnc []byte
	if emp.Salary != nil {
		enc, err := crypto.EncryptString(strconv.FormatFloat(*emp.Salary, 'f', 2, 64))
		if err != nil {
			return nil, nil, nil, err
		}
		salaryEnc = enc
	}
	return nationalEnc, bankEnc, salaryEnc, nil
}

func decryptStringFallback(crypto *cryptoutil.Service, encrypted []byte, plain string) string {
	if crypto == nil || !crypto.Configured() || len(encrypted) == 0 {
		return plain
	}
	decrypted, err := crypto.DecryptString(encrypted)
	if err != nil {
		return plain
	}
	return decrypted
}

func decryptFloatFallback(crypto *cryptoutil.Service, encrypted []byte, plain *float64) *float64 {
	if crypto == nil || !crypto.Configured() || len(encrypted) == 0 {
		return plain
	}
	value, err := crypto.DecryptString(encrypted)
	if err != nil {
		return plain
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return plain
	}
	return &parsed
}
