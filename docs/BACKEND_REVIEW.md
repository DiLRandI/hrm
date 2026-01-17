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

## Findings (Key Gaps)
### Security & Access Control
- Sensitive fields (salary, bank account, national ID) are stored in plaintext; no field-level encryption.
- Password reset tokens are hashed, but session invalidation/rotation policies are minimal.
- Rate limiting is applied only to login/reset endpoints; other sensitive actions lack throttling.

### Data Integrity & Transactions
- Multi-step workflows (e.g., payroll finalize, anonymization) are not wrapped in a single DB transaction, which can lead to partial updates on failure.
- Idempotency is used for payroll finalize and CSV imports only; other sensitive operations (GDPR retention/anonymization) are not idempotent.

### Validation & Input Hygiene
- Many handlers accept JSON payloads with minimal validation (enum values, numeric bounds, date ranges).
- CSV import relies on best-effort parsing with limited schema validation.

### Performance & Scalability
- List endpoints are unbounded; no pagination for large datasets (employees, access logs, notifications, audit, reports).
- Some operations run per-employee loops with multiple queries (payroll run), which can be expensive at scale.
- Missing indexes for common filters (tenant_id + foreign keys) may cause slow scans as data grows.

### Observability & Ops
- No metrics or tracing endpoints; logs are present but not aggregated by request outcome.
- Background jobs (leave accrual, retention) are manual; no scheduler or job status API/UI beyond summary responses.

## Recommendations (Prioritized)
1. Add field-level encryption for sensitive employee data using a managed key (KMS/ENV) and document rotation.
2. Introduce transactional boundaries for multi-step workflows (payroll finalize, anonymization).
3. Add pagination and filters to all list endpoints; add indexes on tenant_id + foreign keys used in filters.
4. Centralize validation with typed request structs + enum validation helpers.
5. Add metrics/tracing endpoints and structured audit log access API.

## Testing Notes
- Unit tests cover some domains; integration tests for authorization scoping and BOLA checks are still light.
