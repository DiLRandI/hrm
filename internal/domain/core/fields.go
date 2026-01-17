package core

import "hrm/internal/domain/auth"

func FilterEmployeeFields(emp *Employee, user auth.UserContext, isSelf, isManager bool) {
	if user.RoleName == "HR" {
		return
	}

	if user.RoleName == "Manager" && (isSelf || isManager) {
		emp.NationalID = ""
		emp.BankAccount = ""
		emp.Salary = nil
		return
	}

	if user.RoleName == "Employee" && isSelf {
		emp.NationalID = ""
		emp.BankAccount = ""
		emp.Salary = nil
		return
	}

	emp.NationalID = ""
	emp.BankAccount = ""
	emp.Salary = nil
}
