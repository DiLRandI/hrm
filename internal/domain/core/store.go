package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	DB *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{DB: db}
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
           COALESCE(bank_account, ''),
           salary,
           currency,
           COALESCE(employment_type, ''),
           COALESCE(department_id::text, ''),
           COALESCE(manager_id::text, ''),
           start_date, end_date, status, created_at, updated_at
    FROM employees
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, employeeID)

	var emp Employee
	err := row.Scan(
		&emp.ID, &emp.UserID, &emp.EmployeeNumber, &emp.FirstName, &emp.LastName, &emp.Email, &emp.Phone,
		&emp.DateOfBirth, &emp.Address, &emp.NationalID, &emp.BankAccount, &emp.Salary, &emp.Currency,
		&emp.EmploymentType, &emp.DepartmentID, &emp.ManagerID, &emp.StartDate, &emp.EndDate, &emp.Status,
		&emp.CreatedAt, &emp.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
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
           COALESCE(bank_account, ''),
           salary,
           currency,
           COALESCE(employment_type, ''),
           COALESCE(department_id::text, ''),
           COALESCE(manager_id::text, ''),
           start_date, end_date, status, created_at, updated_at
    FROM employees
    WHERE tenant_id = $1 AND user_id = $2
  `, tenantID, userID)

	var emp Employee
	if err := row.Scan(
		&emp.ID, &emp.UserID, &emp.EmployeeNumber, &emp.FirstName, &emp.LastName, &emp.Email, &emp.Phone,
		&emp.DateOfBirth, &emp.Address, &emp.NationalID, &emp.BankAccount, &emp.Salary, &emp.Currency,
		&emp.EmploymentType, &emp.DepartmentID, &emp.ManagerID, &emp.StartDate, &emp.EndDate, &emp.Status,
		&emp.CreatedAt, &emp.UpdatedAt,
	); err != nil {
		return nil, err
	}
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
           COALESCE(bank_account, ''),
           salary,
           currency,
           COALESCE(employment_type, ''),
           COALESCE(department_id::text, ''),
           COALESCE(manager_id::text, ''),
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
		if err := rows.Scan(
			&emp.ID, &emp.UserID, &emp.EmployeeNumber, &emp.FirstName, &emp.LastName, &emp.Email, &emp.Phone,
			&emp.DateOfBirth, &emp.Address, &emp.NationalID, &emp.BankAccount, &emp.Salary, &emp.Currency,
			&emp.EmploymentType, &emp.DepartmentID, &emp.ManagerID, &emp.StartDate, &emp.EndDate, &emp.Status,
			&emp.CreatedAt, &emp.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, emp)
	}
	return out, nil
}

func (s *Store) CreateEmployee(ctx context.Context, tenantID string, emp Employee) (string, error) {
	var id string
	err := s.DB.QueryRow(ctx, `
    INSERT INTO employees (tenant_id, user_id, employee_number, first_name, last_name, email, phone, date_of_birth,
      address, national_id, bank_account, salary, currency, employment_type, department_id, manager_id, start_date, end_date, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
    RETURNING id
  `,
		tenantID, nullIfEmpty(emp.UserID), nullIfEmpty(emp.EmployeeNumber), emp.FirstName, emp.LastName, emp.Email, emp.Phone,
		emp.DateOfBirth, emp.Address, emp.NationalID, emp.BankAccount, emp.Salary, emp.Currency, emp.EmploymentType,
		nullIfEmpty(emp.DepartmentID), nullIfEmpty(emp.ManagerID), emp.StartDate, emp.EndDate, emp.Status,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) UpdateEmployee(ctx context.Context, tenantID, employeeID string, emp Employee) error {
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
        bank_account = $9,
        salary = $10,
        currency = $11,
        employment_type = $12,
        department_id = $13,
        manager_id = $14,
        start_date = $15,
        end_date = $16,
        status = $17,
        updated_at = now()
    WHERE tenant_id = $18 AND id = $19
  `,
		emp.EmployeeNumber, emp.FirstName, emp.LastName, emp.Email, emp.Phone, emp.DateOfBirth, emp.Address,
		emp.NationalID, emp.BankAccount, emp.Salary, emp.Currency, emp.EmploymentType,
		nullIfEmpty(emp.DepartmentID), nullIfEmpty(emp.ManagerID), emp.StartDate, emp.EndDate, emp.Status,
		tenantID, employeeID,
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
