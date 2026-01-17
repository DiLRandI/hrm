# Backend Review (Lead Go Engineer)

## Scope
Go backend under `cmd/`, `internal/`, and `migrations/`. Review focuses on architecture, security, data integrity, and operability.

## Architecture Snapshot
- Domain modules under `internal/domain/*` with HTTP handlers in `internal/transport/http/handlers/*`.
- Middleware stack includes request IDs, logging, body limits, auth, rate limiting, and security headers.
- Most business logic executes SQL directly in handlers; only core and notifications have store/service abstractions.

## Strengths
- Consistent API envelope and request ID propagation.
- RBAC permissions are enforced on most routes; scoped filtering is present in leave, payroll, performance, and GDPR.
- Audit logging exists for critical actions (leave, payroll, performance, GDPR).
- Migrations cover core HR, leave, payroll, performance, GDPR, notifications, and idempotency.
- Sensitive fields are encrypted at rest with application-level AES-GCM and key-based configuration.
- Session rotation, MFA, and rate limiting on authentication endpoints are in place.
- Metrics endpoint and job run history are available for ops visibility.

## Findings (Key Gaps)
### Security & Access Control
- Rate limiting is applied only to login/reset endpoints; other sensitive actions lack throttling.

### Data Integrity & Transactions
- Multi-step workflows (e.g., payroll finalize) are not wrapped in a single DB transaction, which can lead to partial updates on failure.
- Idempotency is used for payroll finalize and CSV imports; other sensitive operations (GDPR retention/anonymization) are not idempotent.

### Validation & Input Hygiene
- Many handlers accept JSON payloads with minimal validation (enum values, numeric bounds, date ranges).
- CSV import relies on best-effort parsing with limited schema validation.

### Performance & Scalability
- List endpoints now include pagination for employees, leave requests, audit logs, access logs, payroll lists, and DSAR exports; performance-related lists could still benefit from paging.
- Some operations run per-employee loops with multiple queries (payroll run), which can be expensive at scale.
- Missing indexes for common filters (tenant_id + foreign keys) may cause slow scans as data grows.

### Observability & Ops
- Metrics endpoint exists; tracing is limited to request IDs without distributed tracing.
- Background jobs run on an in-process scheduler with job run history exposed in reports.

## Recommendations (Prioritized)
1. Introduce transactional boundaries for multi-step workflows (payroll finalize, anonymization).
2. Add pagination and filters to remaining large list endpoints; add indexes on tenant_id + foreign keys used in filters.
3. Centralize validation with typed request structs + enum validation helpers.
4. Expand rate limiting to other sensitive operations (imports, approvals).
5. Add distributed tracing or OpenTelemetry export if required.

## Testing Notes
- Unit tests cover domains; integration tests now cover HR journeys and BOLA guards, but more role-specific coverage is still useful.
