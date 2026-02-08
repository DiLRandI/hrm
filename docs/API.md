# API Reference (MVP)

Base path: `/api/v1`

## Health & Ops (non-versioned)
- `GET /healthz`
- `GET /readyz`
- `GET /metrics` (when enabled)

## Auth
- `POST /auth/login` → { email, password }
- `POST /auth/logout`
- `POST /auth/refresh`
- `POST /auth/request-reset` → { email } (returns generic success; sends reset link email when account exists)
- `POST /auth/reset` → { token, newPassword }
- `POST /auth/mfa/setup`
- `POST /auth/mfa/enable`
- `POST /auth/mfa/disable`

## Core HR
- `GET /me`
- `GET /org/chart`
- `GET /employees`
- `POST /employees`
- `GET /employees/{id}`
- `PUT /employees/{id}`
- `GET /employees/{id}/manager-history`
- `GET /departments`
- `POST /departments`
- `PUT /departments/{id}`
- `DELETE /departments/{id}`
- `GET /permissions`
- `GET /roles`
- `PUT /roles/{id}`

## Audit
- `GET /audit/events`
- `GET /audit/events/export`

## Leave
- `GET /leave/types`
- `POST /leave/types`
- `GET /leave/policies`
- `POST /leave/policies`
- `GET /leave/holidays`
- `POST /leave/holidays`
- `DELETE /leave/holidays/{id}`
- `GET /leave/balances`
- `POST /leave/balances/adjust`
- `POST /leave/accrual/run`
- `GET /leave/requests`
- `POST /leave/requests`
- `POST /leave/requests/{id}/approve`
- `POST /leave/requests/{id}/reject`
- `POST /leave/requests/{id}/cancel`
- `GET /leave/calendar`
- `GET /leave/calendar/export`
- `GET /leave/reports/balances`
- `GET /leave/reports/usage`

## Payroll
- `GET /payroll/schedules`
- `POST /payroll/schedules`
- `GET /payroll/groups`
- `POST /payroll/groups`
- `GET /payroll/elements`
- `POST /payroll/elements`
- `GET /payroll/journal-templates`
- `POST /payroll/journal-templates`
- `GET /payroll/periods`
- `POST /payroll/periods`
- `GET /payroll/periods/{id}/inputs`
- `POST /payroll/periods/{id}/inputs`
- `POST /payroll/periods/{id}/inputs/import`
- `GET /payroll/periods/{id}/adjustments`
- `POST /payroll/periods/{id}/adjustments`
- `GET /payroll/periods/{id}/summary`
- `POST /payroll/periods/{id}/run`
- `POST /payroll/periods/{id}/finalize`
- `POST /payroll/periods/{id}/reopen`
- `GET /payroll/periods/{id}/export/register`
- `GET /payroll/periods/{id}/export/journal`
- `GET /payroll/payslips`
- `GET /payroll/payslips/{id}/download`
- `POST /payroll/payslips/{id}/regenerate`

## Performance
- `GET /performance/goals`
- `POST /performance/goals`
- `PUT /performance/goals/{id}`
- `POST /performance/goals/{id}/comments`
- `GET /performance/review-templates`
- `POST /performance/review-templates`
- `GET /performance/review-cycles`
- `POST /performance/review-cycles`
- `POST /performance/review-cycles/{id}/finalize`
- `GET /performance/review-tasks`
- `POST /performance/review-tasks/{id}/responses`
- `GET /performance/feedback`
- `POST /performance/feedback`
- `GET /performance/checkins`
- `POST /performance/checkins`
- `GET /performance/pips`
- `POST /performance/pips`
- `PUT /performance/pips/{id}`
- `GET /performance/reports/summary`

## GDPR
- `GET /gdpr/retention-policies`
- `POST /gdpr/retention-policies`
- `GET /gdpr/retention/runs`
- `POST /gdpr/retention/run`
- `GET /gdpr/consents`
- `POST /gdpr/consents`
- `POST /gdpr/consents/{id}/revoke`
- `GET /gdpr/dsar`
- `POST /gdpr/dsar`
- `GET /gdpr/dsar/{id}/download`
- `POST /gdpr/anonymize`
- `GET /gdpr/anonymize`
- `POST /gdpr/anonymize/{id}/execute`
- `GET /gdpr/anonymize/{id}/download`
- `GET /gdpr/access-logs`

## Reports & Notifications
- `GET /reports/dashboard/employee`
- `GET /reports/dashboard/manager`
- `GET /reports/dashboard/hr`
- `GET /reports/dashboard/employee/export`
- `GET /reports/dashboard/manager/export`
- `GET /reports/dashboard/hr/export`
- `GET /notifications`
- `POST /notifications/{id}/read`
- `GET /notifications/settings`
- `PUT /notifications/settings`
