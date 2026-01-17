-- Security, org, and job enhancements

ALTER TABLE employees ADD COLUMN IF NOT EXISTS national_id_enc BYTEA;
ALTER TABLE employees ADD COLUMN IF NOT EXISTS bank_account_enc BYTEA;
ALTER TABLE employees ADD COLUMN IF NOT EXISTS salary_enc BYTEA;
ALTER TABLE employees ADD COLUMN IF NOT EXISTS pay_group_id UUID REFERENCES pay_groups(id);

ALTER TABLE users ADD COLUMN IF NOT EXISTS mfa_secret_enc BYTEA;

ALTER TABLE review_cycles ADD COLUMN IF NOT EXISTS hr_required BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE leave_policies ADD COLUMN IF NOT EXISTS requires_hr_approval BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE payroll_adjustments ADD COLUMN IF NOT EXISTS effective_date DATE;

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS rotated_at TIMESTAMPTZ;

ALTER TABLE dsar_exports ADD COLUMN IF NOT EXISTS file_path TEXT;
ALTER TABLE dsar_exports ADD COLUMN IF NOT EXISTS file_encrypted BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE dsar_exports ADD COLUMN IF NOT EXISTS download_token TEXT;
ALTER TABLE dsar_exports ADD COLUMN IF NOT EXISTS download_expires_at TIMESTAMPTZ;

ALTER TABLE anonymization_jobs ADD COLUMN IF NOT EXISTS file_path TEXT;
ALTER TABLE anonymization_jobs ADD COLUMN IF NOT EXISTS download_token TEXT;
ALTER TABLE anonymization_jobs ADD COLUMN IF NOT EXISTS download_expires_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS tenant_settings (
  tenant_id UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  email_notifications_enabled BOOLEAN NOT NULL DEFAULT false,
  email_from TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS job_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  job_type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'running',
  details_json JSONB,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS journal_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  config_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
