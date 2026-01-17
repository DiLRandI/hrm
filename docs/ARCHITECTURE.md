# Architecture

## Overview
Single-container application hosting a Go REST API and a React SPA. PostgreSQL runs separately. The Go server serves `/api/v1/*` endpoints and the React `index.html` for SPA routing.

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
