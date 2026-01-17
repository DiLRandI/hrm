CREATE TABLE IF NOT EXISTS retention_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  data_category TEXT NOT NULL,
  cutoff_date TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL DEFAULT 'completed',
  deleted_count INTEGER NOT NULL DEFAULT 0,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_retention_runs_tenant ON retention_runs(tenant_id);
