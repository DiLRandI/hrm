ALTER TABLE departments
  ADD COLUMN IF NOT EXISTS department_code TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS departments_tenant_code_uniq
  ON departments (tenant_id, department_code)
  WHERE department_code IS NOT NULL;
