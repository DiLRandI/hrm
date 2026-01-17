# Implementation Log

Start: 2026-01-17

## Log
- 2026-01-17: Initialized project structure and documentation tracking.
- 2026-01-17: Implemented encryption-at-rest for sensitive fields, MFA, refresh token rotation, audit log export UI, and GDPR consent/retention/anonymization flows with secure downloads.
- 2026-01-17: Added job scheduler (leave accrual + retention), metrics endpoint, email notification support, and job runs dashboard.
- 2026-01-17: Added HR org chart/role management UI, leave HR-approval steps, payroll pay-group-aware runs, journal templates, and performance review templates enforcement.
- 2026-01-17: Added pagination across key list endpoints and UI surfaces (employees, leave requests, DSAR/access logs, audit).
- 2026-01-17: Expanded tests with backend journey coverage, BOLA guard checks, and frontend workflow tests; added Playwright smoke E2E.

## Decisions
- Tenancy: single-tenant per deployment with `tenant_id` column reserved for future multi-tenant support.
- Jobs: in-process scheduler (cron-like ticker) for accruals, retention, and payroll tasks; queue-ready interfaces.
- Payslip PDFs: render server-side HTML → PDF using a pluggable renderer (default: HTML download if no renderer configured).

## Pending
- None.
- 2026-01-17: Created initial database schema migration (core, leave, payroll, performance, GDPR, audit, notifications).
- 2026-01-17: Added password reset table to schema.
- 2026-01-17: Added Go backend skeleton with auth, core HR, leave, payroll, performance, GDPR, notifications, and reports endpoints.
- 2026-01-17: Built React SPA with role-based navigation and core module pages (dashboard, employees, leave, payroll, performance, GDPR, reports, notifications).
- 2026-01-17: Added Dockerfile, docker-compose, and Kubernetes manifests for single-container app with separate Postgres.
- 2026-01-17: Documented architecture, API surface, and deployment steps.
- 2026-01-17: Added Makefile targets for dev, build, test, and Docker workflows.
- 2026-01-17: Added backend unit tests for auth, RBAC permissions, field filtering, leave/day calculation, payroll computation, GDPR payload assembly, reports dashboards, and middleware utilities.
- 2026-01-17: Added frontend unit tests with Vitest and Testing Library (API client, login flow, dashboard fetch).
- 2026-01-17: Refactored Go backend into domain/platform/transport layers and restructured React frontend into app/features/services/shared layout.
- 2026-01-17: Added GDPR retention runs, anonymization execution, and DSAR download support with audit logging.
- 2026-01-17: Added payroll pay groups, adjustments, summary, exports, and CSV import idempotency support.
- 2026-01-17: Added performance review templates and HR finalize endpoint for review cycles.
- 2026-01-17: Expanded frontend pages for leave, payroll, performance, GDPR, and notifications to cover end-to-end workflows.
