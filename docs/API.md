# API Reference

Base path: `/api/v1`

## Health & Ops (non-versioned)
- `GET /healthz`
- `GET /readyz`
- `GET /metrics` (when enabled)

## Auth
- `POST /auth/login` -> `{ email, password, mfaCode? }`
- `POST /auth/logout`
- `POST /auth/refresh`
- `POST /auth/request-reset` -> `{ email }` (always returns generic success)
- `POST /auth/reset` -> `{ token, newPassword }`
- `POST /auth/mfa/setup`
- `POST /auth/mfa/enable`
- `POST /auth/mfa/disable`

## Core HR
- `GET /me`
- `GET /profile/emergency-contacts`
- `PUT /profile/emergency-contacts`
- `GET /org/chart`
- `GET /employees`
- `GET /employees/{employeeID}`
- `PUT /employees/{employeeID}`
- `GET /employees/{employeeID}/emergency-contacts`
- `PUT /employees/{employeeID}/emergency-contacts`
- `GET /employees/{employeeID}/manager-history`
- `GET /departments`
- `POST /departments`
- `PUT /departments/{departmentID}`
- `DELETE /departments/{departmentID}`
- `GET /permissions`
- `GET /roles`
- `PUT /roles/{roleID}`
- `GET /users`
- `POST /users` -> `{ email, role, status?, employee? }`
- `PUT /users/{userID}/role` -> `{ role }`
- `PUT /users/{userID}/status` -> `{ status }`

Employee onboarding uses `POST /users` with `role=Employee` and an `employee` payload.

Current role set:
- `SystemAdmin`
- `Admin`
- `HR`
- `HRManager`
- `Manager`
- `Employee`

Role provisioning policy:
- `SystemAdmin` can create: `Admin`, `HR`, `HRManager`, `Manager`
- `Admin` can create: `HRManager`, `HR`, `Manager`
- `HRManager` can create: `HR`, `Employee`
- `HR` can create: `Employee`

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
- `DELETE /leave/holidays/{holidayID}`
- `GET /leave/balances`
- `POST /leave/balances/adjust`
- `POST /leave/accrual/run`
- `GET /leave/requests`
- `GET /leave/requests/{requestID}`
- `POST /leave/requests`
- `POST /leave/requests/{requestID}/documents` (multipart `documents[]`)
- `GET /leave/requests/{requestID}/documents/{documentID}/download`
- `POST /leave/requests/{requestID}/approve`
- `POST /leave/requests/{requestID}/reject`
- `POST /leave/requests/{requestID}/cancel`
- `GET /leave/calendar`
- `GET /leave/calendar/export`
- `GET /leave/reports/balances`
- `GET /leave/reports/usage`

`POST /leave/requests` supports `startHalf`/`endHalf` and can accept multipart form-data with uploaded `documents` for leave types requiring evidence.

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
- `GET /payroll/periods/{periodID}/inputs`
- `POST /payroll/periods/{periodID}/inputs`
- `POST /payroll/periods/{periodID}/inputs/import` (supports optional `Idempotency-Key`)
- `GET /payroll/periods/{periodID}/adjustments`
- `POST /payroll/periods/{periodID}/adjustments`
- `GET /payroll/periods/{periodID}/summary`
- `POST /payroll/periods/{periodID}/run`
- `POST /payroll/periods/{periodID}/finalize` (requires `Idempotency-Key`)
- `POST /payroll/periods/{periodID}/reopen`
- `GET /payroll/periods/{periodID}/export/register`
- `GET /payroll/periods/{periodID}/export/journal`
- `GET /payroll/payslips`
- `GET /payroll/payslips/{payslipID}/download`
- `POST /payroll/payslips/{payslipID}/regenerate`

## Performance
- `GET /performance/goals`
- `POST /performance/goals`
- `PUT /performance/goals/{goalID}`
- `POST /performance/goals/{goalID}/comments`
- `GET /performance/review-templates`
- `POST /performance/review-templates`
- `GET /performance/review-cycles`
- `POST /performance/review-cycles`
- `POST /performance/review-cycles/{cycleID}/finalize`
- `GET /performance/review-tasks`
- `POST /performance/review-tasks/{taskID}/responses`
- `GET /performance/feedback`
- `POST /performance/feedback`
- `GET /performance/checkins`
- `POST /performance/checkins`
- `GET /performance/pips`
- `POST /performance/pips`
- `PUT /performance/pips/{pipID}`
- `GET /performance/reports/summary`

## GDPR
- `GET /gdpr/retention-policies`
- `POST /gdpr/retention-policies`
- `GET /gdpr/retention/runs`
- `POST /gdpr/retention/run`
- `GET /gdpr/consents`
- `POST /gdpr/consents`
- `POST /gdpr/consents/{consentID}/revoke`
- `GET /gdpr/dsar`
- `POST /gdpr/dsar`
- `GET /gdpr/dsar/{exportID}/download`
- `GET /gdpr/anonymize`
- `POST /gdpr/anonymize`
- `POST /gdpr/anonymize/{jobID}/execute`
- `GET /gdpr/anonymize/{jobID}/download`
- `GET /gdpr/access-logs`

## Reports & Notifications
- `GET /reports/dashboard/employee`
- `GET /reports/dashboard/manager`
- `GET /reports/dashboard/hr`
- `GET /reports/dashboard/employee/export`
- `GET /reports/dashboard/manager/export`
- `GET /reports/dashboard/hr/export`
- `GET /reports/jobs` (`jobType`, `status`, `startedFrom`, `startedTo`, pagination + `X-Total-Count`)
- `GET /reports/jobs/{runID}`
- `GET /notifications`
- `POST /notifications/{notificationID}/read`
- `GET /notifications/settings`
- `PUT /notifications/settings`

## Error and Response Conventions
- Validation failures return code `validation_error` with field-level entries in `error.details.fields[]`.
- Idempotent operations return `idempotency_conflict` (`409`) when a key is reused with a different request hash.
- Sensitive mutation throttles return `429` with `Retry-After`, `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset`.
- Many list endpoints use `limit` and `offset`; paginated responses include `X-Total-Count`.
