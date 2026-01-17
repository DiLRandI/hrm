# Pending Features & Gaps

This document tracks remaining gaps against the PRD and production readiness review after the latest implementation pass.

## Security & Compliance
- **Encryption at rest** for sensitive fields (salary, bank account, national ID) is not implemented at the application or DB layer. A key-management plan and field-level encryption are still needed.
- **MFA** is not implemented (optional in v1 but still pending).
- **Refresh token rotation** and explicit session revocation flows are minimal; logout clears sessions but there is no rotation policy.
- **Audit log access**: audit events exist but there is no API/UI for audit log review.

## HR Core & Org
- **Department maintenance** supports list/create only; update/delete flows and org chart visualization are pending.
- **Manager history UI** is not exposed (history is tracked in `manager_relations` but not surfaced).

## Reporting & Exports
- **Cross-module report exports** (CSV/PDF) beyond payroll/leave exports are limited. Reports dashboards do not support export.

## Observability & Ops
- **Metrics and tracing** endpoints are not implemented.
- **Background job status pages** (accrual/retention/payroll) are not exposed beyond API responses.

## Testing
- **End-to-end UI tests** (Playwright) are not implemented.
- **Integration tests** for authorization and BOLA coverage are minimal.

These items should be reviewed and scheduled based on deployment risk and compliance requirements.
