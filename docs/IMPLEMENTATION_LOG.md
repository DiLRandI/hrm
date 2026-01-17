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

## Decisions
- Tenancy: single-tenant per deployment with `tenant_id` column reserved for future multi-tenant support.
- Jobs: in-process scheduler (cron-like ticker) for accruals, retention, and payroll tasks; queue-ready interfaces.
- Payslip PDFs: render server-side HTML → PDF using a pluggable renderer (default: HTML download if no renderer configured).

## Pending
- Broader rate limiting for non-auth sensitive endpoints (imports, approvals, GDPR actions).
- Structured payload validation for enums/bounds/date ranges.
- Wrap payroll finalize in a transaction and add idempotency for GDPR retention/anonymization.
- Expand unit/E2E tests for performance and notifications workflows.
