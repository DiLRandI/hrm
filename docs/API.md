# API Reference (MVP)

Base path: `/api/v1`

## Auth
- `POST /auth/login` → { email, password }
- `POST /auth/logout`
- `POST /auth/request-reset` → { email }
- `POST /auth/reset` → { token, newPassword }

## Core HR
- `GET /me`
- `GET /employees`
- `POST /employees`
- `GET /employees/{id}`
- `PUT /employees/{id}`
- `GET /departments`
- `POST /departments`

## Leave
- `GET /leave/types`
- `POST /leave/types`
- `GET /leave/policies`
- `POST /leave/policies`
- `GET /leave/balances`
- `POST /leave/balances/adjust`
- `GET /leave/requests`
- `POST /leave/requests`
- `POST /leave/requests/{id}/approve`
- `POST /leave/requests/{id}/reject`
- `POST /leave/requests/{id}/cancel`
- `GET /leave/calendar`
- `GET /leave/reports/balances`
- `GET /leave/reports/usage`

## Payroll
- `GET /payroll/schedules`
- `POST /payroll/schedules`
- `GET /payroll/elements`
- `POST /payroll/elements`
- `GET /payroll/periods`
- `POST /payroll/periods`
- `GET /payroll/periods/{id}/inputs`
- `POST /payroll/periods/{id}/inputs`
- `POST /payroll/periods/{id}/run`
- `POST /payroll/periods/{id}/finalize`
- `GET /payroll/payslips`

## Performance
- `GET /performance/goals`
- `POST /performance/goals`
- `POST /performance/goals/{id}/comments`
- `GET /performance/review-cycles`
- `POST /performance/review-cycles`
- `GET /performance/review-tasks`
- `POST /performance/review-tasks/{id}/responses`
- `GET /performance/feedback`
- `POST /performance/feedback`
- `GET /performance/checkins`
- `POST /performance/checkins`
- `GET /performance/pips`
- `POST /performance/pips`

## GDPR
- `GET /gdpr/retention-policies`
- `POST /gdpr/retention-policies`
- `GET /gdpr/dsar`
- `POST /gdpr/dsar`
- `POST /gdpr/anonymize`
- `GET /gdpr/access-logs`

## Reports & Notifications
- `GET /reports/dashboard/employee`
- `GET /reports/dashboard/manager`
- `GET /reports/dashboard/hr`
- `GET /notifications`
- `POST /notifications/{id}/read`
