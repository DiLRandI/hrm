-- Profile additions: personal fields + emergency contacts

ALTER TABLE employees ADD COLUMN IF NOT EXISTS preferred_name TEXT;
ALTER TABLE employees ADD COLUMN IF NOT EXISTS personal_email TEXT;
ALTER TABLE employees ADD COLUMN IF NOT EXISTS pronouns TEXT;

CREATE TABLE IF NOT EXISTS employee_emergency_contacts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  full_name TEXT NOT NULL,
  relationship TEXT NOT NULL,
  phone TEXT,
  email TEXT,
  address TEXT,
  is_primary BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_emergency_contacts_employee ON employee_emergency_contacts(employee_id);
CREATE INDEX IF NOT EXISTS idx_emergency_contacts_tenant ON employee_emergency_contacts(tenant_id);
