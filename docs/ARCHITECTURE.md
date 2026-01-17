# Architecture

## Overview
Single-container application hosting a Go REST API and a React SPA. PostgreSQL runs separately. The Go server serves `/api/v1/*` endpoints and the React `index.html` for SPA routing.

## Repository layout (refactored)

Backend (Go):
- `cmd/server`: entrypoint binary.
- `internal/app`: server wiring and routing.
- `internal/domain`: business logic + domain models.
- `internal/platform`: config, database, request context.
- `internal/transport/http`: HTTP handlers, middleware, API helpers.

Frontend (React):
- `frontend/src/app`: app shell and routing.
- `frontend/src/features`: feature modules by domain.
- `frontend/src/services`: API client and external integrations.
- `frontend/src/shared`: shared styles and UI helpers.

## Modules
- Core HR: employees, departments, org structure, RBAC, access logs.
- Leave: policies, requests, balances, approvals, calendar.
- Payroll: schedules, inputs, calculation, finalize, payslips.
- Performance: goals, review cycles/tasks, feedback, check-ins, PIP.
- GDPR: DSAR export, retention policies, anonymization requests.

## Security
- JWT-based auth with role-based access control.
- Field-level filtering for sensitive employee fields.
- Audit-ready access logs and immutable audit event support.

## Jobs
In-process jobs can be added for accruals, retention, and payroll recalculation. The design is queue-ready for future background worker integration.

## Observability
- Request IDs in logs plus JSON logging.
- Optional metrics endpoint for basic latency/error/job counters.
