# Implementation Log

Start: 2026-01-17

## Log
- 2026-01-17: Initialized project structure and documentation tracking.
- 2026-01-17: Implemented encryption-at-rest for sensitive fields, MFA, refresh token rotation, audit log export UI, and GDPR consent/retention/anonymization flows with secure downloads.
- 2026-01-17: Added job scheduler (leave accrual + retention), metrics endpoint, email notification support, and job runs dashboard.
- 2026-01-17: Added HR org chart/role management UI, leave HR-approval steps, payroll pay-group-aware runs, journal templates, and performance review templates enforcement.
- 2026-01-17: Added pagination across key list endpoints and UI surfaces (employees, leave requests, DSAR/access logs, audit).
- 2026-01-17: Expanded tests with backend journey coverage, BOLA guard checks, and frontend workflow tests; added Playwright smoke E2E.
- 2026-01-17: Added toast notifications and HR-only route guard for the audit screen.
- 2026-01-17: Began backend refactor by moving audit and notifications DB access into domain services.
- 2026-01-17: Continued service/store refactor across payroll, performance, and GDPR handlers; moved payslip PDF generation to payroll service; routed audit logging through shared audit service; removed transport-level DB access; fixed accrual job handling to avoid import cycles.
- 2026-02-08: Hardened sensitive mutation abuse protection with endpoint-scoped throttling, auth email+IP controls, `Retry-After` metadata, and rate-limited metrics tracking.
- 2026-02-08: Added shared structured payload validation (`validation_error` with field details) and applied it to high-risk leave, payroll, GDPR, and auth reset write paths.
- 2026-02-08: Converted payroll finalization to a transactional lock-and-finalize flow with in-transaction payslip creation, required idempotency key, conflict-safe key/hash enforcement, and post-commit side effects.
- 2026-02-08: Fixed reports job-runs query indexing, added status/date filters, total-count pagination metadata, single run detail endpoint, and reports UI detail rendering.
- 2026-02-08: Added targeted backend/frontend test coverage for new rate limit behavior, validation shape, payroll idempotency/finalize paths, and reports jobs filtering/details.

## Decisions
- Tenancy: single-tenant per deployment with `tenant_id` column reserved for future multi-tenant support.
- Jobs: in-process scheduler (cron-like ticker) for accruals, retention, and payroll tasks; queue-ready interfaces.
- Payslip PDFs: render server-side HTML → PDF using a pluggable renderer (default: HTML download if no renderer configured).

## Pending
- Expand unit/E2E tests for performance and notifications workflows.
