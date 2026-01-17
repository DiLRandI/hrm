package payroll

type InputLine struct {
	Type   string
	Amount float64
}

func ComputePayroll(baseSalary float64, inputs []InputLine) (gross, deductions, net float64) {
	gross = baseSalary
	for _, input := range inputs {
		switch input.Type {
		case "earning":
			gross += input.Amount
		case "deduction":
			deductions += input.Amount
		}
	}
	net = gross - deductions
	return gross, deductions, net
}
