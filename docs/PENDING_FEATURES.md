# Pending Features & Gaps (Full Analysis)

This document lists remaining gaps against the PRD and production-readiness expectations. Items are grouped by area and ordered by impact.

## Security & Compliance
- **Encryption at rest** for sensitive fields (salary, bank account, national ID) is not implemented (DB/application-level encryption missing).
- **Audit log access**: audit events exist in DB but there is no API/UI for review or export.
- **MFA** is not implemented (optional in v1 but still pending).
- **Refresh token rotation** and token revocation policy are minimal (logout clears a session record but no rotation policy).
- **Security headers** are basic; CSP tuning and HSTS enforcement depend on upstream TLS and are not fully configured.

## GDPR
- **DSAR export coverage is partial**: exports include employee, leave requests, payroll results, and goals only; missing feedback/check-ins/PIPs/notifications/access logs.
- **Consent records** table exists, but there are no API endpoints or UI flows to manage consent.
- **Retention jobs are manual**: no scheduler/cron to run retention automatically; no run history UI beyond API list.
- **Anonymization storage** relies on local file paths; no secure object storage or signed download URLs.

## Core HR & Org
- **Department update/delete** flows are missing (only list/create).
- **Org chart UI** and manager history UI are not present (history is stored in `manager_relations` only).
- **Role/permission management UI** is missing (roles seeded only).

## Leave
- **Accruals run manually** via API; no scheduled accrual job.
- **Approval rules** are simplified; policy-driven multi-step approval is not configurable in UI.

## Payroll
- **Pay groups are not applied**: there is no employee-to-pay-group assignment and payroll does not use groups for currency/schedule overrides.
- **Retro adjustments** are basic (single-line adjustments only; no effective-date calculations).
- **Journal template export** is a basic CSV; no configurable templates or mapping rules.

## Performance
- **Review templates** are stored but not enforced in review response UI (responses are free-form JSON).
- **Review cycle workflow** is simplified; no task state transitions beyond submit/finalize.
- **Rating distribution dashboards** are limited to summary numbers; no visual breakdown UI.

## Reporting & Exports
- **Cross-module exports** (CSV/PDF) are limited to payroll and leave calendar; reports dashboards are not exportable.
- **Pagination** is missing for large datasets (employees, leave requests, audit/access logs, etc.).

## Notifications
- **Email notifications** are not implemented (in-app only).

## Observability & Ops
- **Metrics/tracing** endpoints are missing.
- **Job status pages** for payroll/accrual/retention are not implemented (API-only summaries).
- **Background job queue** is not implemented (all jobs are in-process/manual).

## Testing
- **E2E UI tests** (Playwright) are not present.
- **Authorization coverage** in integration tests is minimal; BOLA checks should be expanded.

These items should be prioritized based on deployment risk (security/compliance first), followed by operations and scalability concerns.
