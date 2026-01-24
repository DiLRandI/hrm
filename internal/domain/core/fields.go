package core

import "hrm/internal/domain/auth"

func FilterEmployeeFields(emp *Employee, user auth.UserContext, isSelf, isManager bool) {
	if user.RoleName == auth.RoleHR {
		return
	}

	if user.RoleName == auth.RoleManager && (isSelf || isManager) {
		emp.NationalID = ""
		emp.BankAccount = ""
		emp.Salary = nil
		if !isSelf {
			emp.PersonalEmail = ""
		}
		return
	}

	if user.RoleName == auth.RoleEmployee && isSelf {
		emp.NationalID = ""
		emp.BankAccount = ""
		emp.Salary = nil
		return
	}

	emp.NationalID = ""
	emp.BankAccount = ""
	emp.Salary = nil
	emp.PersonalEmail = ""
}
