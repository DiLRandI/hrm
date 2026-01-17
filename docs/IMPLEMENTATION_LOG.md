# Implementation Log

Start: 2026-01-17

## Log
- 2026-01-17: Initialized project structure and documentation tracking.

## Decisions
- Tenancy: single-tenant per deployment with `tenant_id` column reserved for future multi-tenant support.
- Jobs: in-process scheduler (cron-like ticker) for accruals, retention, and payroll tasks; queue-ready interfaces.
- Payslip PDFs: render server-side HTML → PDF using a pluggable renderer (default: HTML download if no renderer configured).

## Pending
- Define full schema/migrations for core, leave, payroll, performance, GDPR, audit, notifications.
- Implement Go API server and middleware.
- Build React SPA and API integration.
- Add Docker/Docker Compose/K8s assets and README.
- 2026-01-17: Created initial database schema migration (core, leave, payroll, performance, GDPR, audit, notifications).
- 2026-01-17: Added password reset table to schema.
- 2026-01-17: Added Go backend skeleton with auth, core HR, leave, payroll, performance, GDPR, notifications, and reports endpoints.
- 2026-01-17: Built React SPA with role-based navigation and core module pages (dashboard, employees, leave, payroll, performance, GDPR, reports, notifications).
- 2026-01-17: Added Dockerfile, docker-compose, and Kubernetes manifests for single-container app with separate Postgres.
- 2026-01-17: Documented architecture, API surface, and deployment steps.
- 2026-01-17: Added Makefile targets for dev, build, test, and Docker workflows.
- 2026-01-17: Added backend unit tests for auth, RBAC permissions, field filtering, leave/day calculation, payroll computation, GDPR payload assembly, reports dashboards, and middleware utilities.
- 2026-01-17: Added frontend unit tests with Vitest and Testing Library (API client, login flow, dashboard fetch).
