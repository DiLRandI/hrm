package payroll

import "testing"

func TestComputePayroll(t *testing.T) {
	inputs := []InputLine{
		{Type: "earning", Amount: 200},
		{Type: "earning", Amount: 50},
		{Type: "deduction", Amount: 100},
	}

	gross, deductions, net := ComputePayroll(1000, inputs)
	if gross != 1250 {
		t.Fatalf("expected gross 1250, got %v", gross)
	}
	if deductions != 100 {
		t.Fatalf("expected deductions 100, got %v", deductions)
	}
	if net != 1150 {
		t.Fatalf("expected net 1150, got %v", net)
	}
}

func TestComputePayrollIgnoresUnknownTypes(t *testing.T) {
	inputs := []InputLine{
		{Type: "bonus", Amount: 100},
		{Type: "deduction", Amount: 25},
	}
	gross, deductions, net := ComputePayroll(500, inputs)
	if gross != 500 {
		t.Fatalf("expected gross 500, got %v", gross)
	}
	if deductions != 25 {
		t.Fatalf("expected deductions 25, got %v", deductions)
	}
	if net != 475 {
		t.Fatalf("expected net 475, got %v", net)
	}
}
