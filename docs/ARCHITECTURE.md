# Architecture

## Overview
The service is a single Go process that:
- serves REST APIs under `/api/v1`
- serves the built React SPA for browser routes
- manages in-process background jobs (leave accrual, GDPR retention, payroll-related job tracking)

PostgreSQL is a separate service.

## Repository Layout

Backend (Go):
- `cmd/server`: process entrypoint
- `internal/app`: server wiring, dependency construction, router assembly
- `internal/domain`: business logic, stores, constants, tests
- `internal/platform`: config, DB, crypto, email, metrics, jobs, request context
- `internal/transport/http`: HTTP handlers, middleware, response helpers

Frontend (React + Vite):
- `frontend/src/app`: app shell, routes, guards
- `frontend/src/features`: domain features (`auth`, `core`, `leave`, `payroll`, `performance`, `gdpr`, `reports`, `notifications`, `audit`, `profile`)
- `frontend/src/services`: API client layer
- `frontend/src/shared`: shared UI/components/hooks/constants/styles
- `frontend/e2e`: Playwright smoke tests

## Domain Modules
- Auth: login/logout/refresh, password reset flow, MFA setup/enable/disable
- Core HR: employees, departments, org chart, role/permission administration, emergency contacts
- Leave: leave types/policies/holidays, requests and approvals, balances/accruals, documents, reports
- Payroll: schedules/groups/elements/periods, inputs/import, run/finalize/reopen, payslips and exports
- Performance: goals, reviews/cycles/tasks, feedback, check-ins, PIPs, summary reporting
- GDPR: retention policies/runs, consent, DSAR export, anonymization, access logs
- Reports: dashboards and job-runs operational reporting
- Notifications and audit trails

## Request Pipeline
Global middleware stack includes:
- request IDs
- structured request logging and metrics capture
- panic recovery
- security headers
- JWT auth context loading

API-scoped middleware adds:
- body size limits
- sensitive mutation rate limiting

## Security Model
- JWT auth with permission-based RBAC checks
- application-level encryption support for sensitive fields (`DATA_ENCRYPTION_KEY`)
- role-aware field filtering for sensitive employee data
- audit events and GDPR access-log recording for sensitive operations
- idempotency enforcement for compliance-critical mutations (for example payroll finalization)
- hierarchical role provisioning controls (`SystemAdmin` -> `Admin`/`HR`/`HRManager`/`Manager`, `Admin` -> `HRManager`/`HR`/`Manager`, `HRManager` -> `HR`/`Employee`, `HR` -> `Employee`)

## Observability
- JSON structured logs
- readiness (`/readyz`) and liveness (`/healthz`) endpoints
- optional `/metrics` endpoint with request and job counters
