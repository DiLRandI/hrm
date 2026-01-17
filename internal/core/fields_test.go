package core

import (
	"testing"

	"hrm/internal/auth"
)

func sampleEmployee() *Employee {
	salary := 120000.0
	return &Employee{
		NationalID:  "ID123",
		BankAccount: "BANK123",
		Salary:      &salary,
	}
}

func TestFilterEmployeeFieldsHR(t *testing.T) {
	emp := sampleEmployee()
	user := auth.UserContext{RoleName: "HR"}

	FilterEmployeeFields(emp, user, false, false)

	if emp.NationalID == "" || emp.BankAccount == "" || emp.Salary == nil {
		t.Fatal("HR should retain sensitive fields")
	}
}

func TestFilterEmployeeFieldsManager(t *testing.T) {
	emp := sampleEmployee()
	user := auth.UserContext{RoleName: "Manager"}

	FilterEmployeeFields(emp, user, false, true)

	if emp.NationalID != "" || emp.BankAccount != "" || emp.Salary != nil {
		t.Fatal("Manager should not see sensitive fields")
	}
}

func TestFilterEmployeeFieldsEmployeeSelf(t *testing.T) {
	emp := sampleEmployee()
	user := auth.UserContext{RoleName: "Employee"}

	FilterEmployeeFields(emp, user, true, false)

	if emp.NationalID != "" || emp.BankAccount != "" || emp.Salary != nil {
		t.Fatal("Employee should not see sensitive fields")
	}
}
