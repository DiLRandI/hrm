package core

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	cryptoutil "hrm/internal/platform/crypto"
)

type Store struct {
	DB     *pgxpool.Pool
	Crypto *cryptoutil.Service
}

func NewStore(db *pgxpool.Pool, crypto *cryptoutil.Service) *Store {
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
	nationalEnc, bankEnc, salaryEnc := encryptEmployeeSensitive(s.Crypto, emp)
	var nationalPlain, bankPlain any = emp.NationalID, emp.BankAccount
	var salaryPlain any = emp.Salary
	if s.Crypto != nil && s.Crypto.Configured() {
		nationalPlain = nil
		bankPlain = nil
		salaryPlain = nil
	}
	var id string
	err := s.DB.QueryRow(ctx, `
    INSERT INTO employees (tenant_id, user_id, employee_number, first_name, last_name, email, phone, date_of_birth,
      address, national_id, national_id_enc, bank_account, bank_account_enc, salary, salary_enc, currency,
      employment_type, department_id, manager_id, pay_group_id, start_date, end_date, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
    RETURNING id
  `,
		tenantID, nullIfEmpty(emp.UserID), nullIfEmpty(emp.EmployeeNumber), emp.FirstName, emp.LastName, emp.Email, emp.Phone,
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
	nationalEnc, bankEnc, salaryEnc := encryptEmployeeSensitive(s.Crypto, emp)
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

func encryptEmployeeSensitive(crypto *cryptoutil.Service, emp Employee) ([]byte, []byte, []byte) {
	if crypto == nil || !crypto.Configured() {
		return nil, nil, nil
	}
	nationalEnc, _ := crypto.EncryptString(emp.NationalID)
	bankEnc, _ := crypto.EncryptString(emp.BankAccount)
	var salaryEnc []byte
	if emp.Salary != nil {
		salaryEnc, _ = crypto.EncryptString(strconv.FormatFloat(*emp.Salary, 'f', 2, 64))
	}
	return nationalEnc, bankEnc, salaryEnc
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
