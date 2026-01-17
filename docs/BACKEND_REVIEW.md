# Backend Review (Lead Go Engineer)

## Architecture Snapshot
- Clear domain segmentation under `internal/domain/*` with HTTP handlers in `internal/transport/http/handlers/*`.
- Middleware stack covers request IDs, logging, body limits, rate limits, auth, and security headers.
- SQL is performed directly in handlers for many operations; a few domain stores exist (core, notifications).

## Strengths
- Consistent API envelope and request ID propagation.
- Good use of role-based permissions and scoped filtering in most handlers.
- Audit logging exists for critical actions (leave, payroll, performance, GDPR).
- Migrations cover core HR, leave, payroll, performance, GDPR, notifications, and idempotency.

## Risks & Gaps
- **Business logic in handlers:** direct SQL in handlers makes reuse/testing harder and complicates transactions.
- **Validation gaps:** most payloads are minimally validated; enum and range validation is inconsistent.
- **Encryption at rest:** sensitive fields are stored in plaintext.
- **Observability:** no metrics/tracing endpoints or job status pages.
- **Pagination:** list endpoints are unbounded and can grow large.

## Recommendations
1. Introduce domain stores/services per module (leave/payroll/performance/gdpr) and move SQL out of handlers.
2. Add request validation helpers (enum validation, date parsing, numeric bounds) for each module.
3. Implement field-level encryption for sensitive employee data with a managed key.
4. Add pagination + filters for list endpoints, especially reports and audit/access logs.
5. Add metrics (latency, DB errors, job duration) and structured tracing for production readiness.

## Testing
- Unit coverage exists for several domains but integration coverage for authorization and data-scoping should be expanded.
