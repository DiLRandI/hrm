# Immediate Priority Implementation Checklist

Date: 2026-02-08  
Scope: Selected urgent requirements for immediate execution

## 1) Priority sequence (recommended)
This sequence reduces risk and rework:
1. Complete password-reset delivery flow
2. Broader abuse protection on sensitive mutation endpoints
3. Structured payload validation for enums/ranges/date logic
4. Transactional payroll finalization and hard idempotency
5. Leave workflow completeness
6. Reliable operations monitoring endpoints

Reasoning:
- Items 2 and 3 are cross-cutting controls that should be in place before expanding behavior.
- Item 4 is financial/compliance-critical and should be stabilized before new leave complexity.
- Item 6 depends on reliable job and finalization behavior for meaningful monitoring.

## 2) Suggested sprint plan (immediate)
- Sprint A (2026-02-09 to 2026-02-13): Items 1, 2, 3 foundation.
- Sprint B (2026-02-16 to 2026-02-20): Item 4.
- Sprint C (2026-02-23 to 2026-02-27): Items 5 and 6.

---

## 3) Item checklists with implementation guidance

## A. Complete password-reset delivery flow
Status (2026-02-08): Complete (implementation and integration tests in place)

### A1. Checklist
- [x] Add configurable frontend reset URL base (for link generation).
- [x] Keep generic API response to prevent account enumeration.
- [x] Send reset email containing one-time token link.
- [x] Keep storing only token hash in DB; never persist raw token.
- [x] Enforce token TTL and one-time usage (already partial; verify end-to-end).
- [x] Add password strength validation on reset.
- [x] Add audit event(s) for request and successful reset.
- [x] Add tests for request, delivery path, and reset success/failure.
- [x] UX: Use token from reset link by default, allow override, and show password requirements in UI.

### A2. Implementation guidance
- Primary files:
  - `internal/transport/http/handlers/auth/handlers.go`
  - `internal/domain/auth/service.go`
  - `internal/domain/auth/store.go`
  - `internal/platform/config/config.go`
  - `docs/DEPLOYMENT.md`
  - `frontend/src/features/auth/pages/RequestReset.jsx`
- Add config keys such as:
  - `FRONTEND_BASE_URL` (example: `https://hr.example.com`)
  - `PASSWORD_RESET_TTL` (default `2h`)
- Link format recommendation:
  - `${FRONTEND_BASE_URL}/reset?token=<rawToken>`
- Delivery recommendation:
  - Use existing SMTP mailer integration (`internal/platform/email/email.go`) with a dedicated reset email template.
- Security notes:
  - Return the same success response for existing and non-existing emails.
  - Do not log raw reset token.
  - Add basic reset request throttling.

### A3. Acceptance criteria
- User receives reset email with valid link.
- Token expires at configured TTL and cannot be reused after successful reset.
- Invalid/expired token returns deterministic validation error.
- Password reset works without exposing whether account exists.

### A4. Test checklist
- [x] Unit: password validation and reset-link/message helper behavior.
- [x] Integration: `request-reset` -> email dispatch mocked -> `reset`.
- [x] Negative: invalid token, expired token, reused token.
- [x] Negative: weak password rejected.
- [x] Negative: unknown email on `request-reset` returns generic success without delivery.
- [x] Frontend: reset page token-link behavior and client-side validation.
- Note: backend integration reset tests are DB-backed and run when `TEST_DATABASE_URL` is configured.

---

## B. Broader abuse protection on sensitive mutation endpoints
Status (2026-02-08): Complete (policy middleware, retry metadata, metrics, and tests implemented)

### B1. Checklist
- [x] Define a sensitive-endpoint rate-limit policy (per user and per IP fallback).
- [x] Apply stricter rate limits to high-risk POST/PUT actions.
- [x] Return `429` consistently with retry metadata.
- [x] Track rate-limited counts in logs/metrics.
- [x] Add tests covering allowed vs throttled behavior.

### B2. Implementation guidance
- Primary files:
  - `internal/app/server/server.go`
  - `internal/transport/http/middleware/ratelimit.go`
  - `internal/transport/http/middleware/auth.go`
  - `internal/platform/config/config.go`
- Endpoints to prioritize:
  - `/auth/request-reset`, `/auth/reset`, MFA endpoints
  - `/leave/requests/*/approve`, `/leave/requests/*/reject`, `/leave/accrual/run`
  - `/payroll/periods/*/run`, `/payroll/periods/*/finalize`, `/payroll/periods/*/inputs/import`
  - `/gdpr/retention/run`, `/gdpr/dsar`, `/gdpr/anonymize/*`
- Policy recommendation:
  - Auth endpoints: strict per-IP and per-email.
  - Authenticated sensitive operations: per-user key first, IP fallback.

### B3. Acceptance criteria
- Sensitive endpoints are demonstrably throttled under burst load.
- Normal user workflows are not blocked under expected usage.
- 429 responses are consistent and observable.

### B4. Test checklist
- [x] Middleware unit tests for keying and window reset.
- [x] Integration tests for representative sensitive routes.
- [x] Regression test that standard read/list routes are unaffected.

---

## C. Structured payload validation for enums/ranges/date logic
Status (2026-02-08): Complete (shared validator + endpoint hardening + tests implemented)

### C1. Checklist
- [x] Introduce a shared validation layer/helper.
- [x] Validate enums against domain constants (statuses, frequencies, element types).
- [x] Validate ranges and bounds (amounts, weights, percentages, day/month limits).
- [x] Validate date logic (`start <= end`, non-zero dates, optional max window rules).
- [x] Return consistent error shape with field-level messages.
- [x] Apply first to high-risk write endpoints.
- [x] Add endpoint-level tests for invalid payload cases.

### C2. Implementation guidance
- Primary files:
  - `internal/transport/http/handlers/leave/handlers.go`
  - `internal/transport/http/handlers/payroll/handlers.go`
  - `internal/transport/http/handlers/performance/handlers.go`
  - `internal/transport/http/handlers/gdpr/handlers.go`
  - `internal/transport/http/handlers/auth/handlers.go`
  - optional new helper: `internal/transport/http/shared/validation.go`
- Start with strict validation on:
  - Payroll period creation, input import rows, adjustments.
  - Leave request create/policy create/balance adjust.
  - GDPR retention policy create/run payload.
  - Auth reset new password payload.
- Error contract recommendation:
  - HTTP 400
  - code: `validation_error`
  - include field + reason list.

### C3. Acceptance criteria
- Invalid enum/range/date payloads are rejected deterministically.
- Validation responses are consistent across modules.
- Existing valid payloads continue to work.

### C4. Test checklist
- [x] Table-driven validation tests for each hardened endpoint.
- [x] Journey tests confirm valid flows unaffected.

---

## D. Transactional payroll finalization and hard idempotency
Status (2026-02-08): Complete (transactional finalize flow, idempotency conflict handling, and tests implemented)

### D1. Checklist
- [x] Move finalize flow into a single transactional service method.
- [x] Lock payroll period row and re-check current state in transaction.
- [x] Create/update payslip rows within the same transaction.
- [x] Commit only after state + records are consistent.
- [x] Harden idempotency:
  - same key + same request hash returns stored response.
  - same key + different request hash returns conflict error.
- [x] Keep external side effects (PDF generation/notifications) post-commit.
- [x] Add rollback-safe failure handling and audit.
- [x] Add integration tests for retry/concurrency paths.

### D2. Implementation guidance
- Primary files:
  - `internal/transport/http/handlers/payroll/handlers.go`
  - `internal/domain/payroll/service.go`
  - `internal/domain/payroll/store*.go`
  - `internal/transport/http/middleware/idempotency.go`
- Current risk to address:
  - Finalize and downstream payslip generation are multi-step and can partially succeed.
  - Current idempotency check can allow same key reuse with different payload hash if not explicitly guarded.
- Recommended behavior:
  - Require `Idempotency-Key` for finalize endpoint.
  - Return `409 conflict` for key/hash mismatch.

### D3. Acceptance criteria
- No partial finalized state is observable after failed finalize.
- Retry with same idempotency key is safe and stable.
- Concurrent finalize attempts resolve deterministically.

### D4. Test checklist
- [x] Integration: finalize success and idempotent replay.
- [x] Integration: same key with different payload -> conflict.
- [x] Integration: induced failure mid-flow -> transaction rollback.
- [x] Integration: concurrent finalize requests.

---

## E. Leave workflow completeness
Status (2026-02-08): Complete (API + UI + tests delivered)

### E1. Checklist
- [x] Support half-day requests (`startHalf`, `endHalf`) in API + UI.
- [x] Update day calculation logic for half-day combinations.
- [x] Enforce supporting documents when leave type requires docs.
- [x] Add request-document upload + metadata persistence.
- [x] Include document/half-day state in leave request list/detail responses.
- [x] Add validation for conflicting/invalid half-day combinations.
- [x] Add tests for half-day math and document-required policy enforcement.

### E2. Implementation guidance
- Primary files:
  - `internal/domain/leave/models.go`
  - `internal/domain/leave/logic.go`
  - `internal/domain/leave/service.go`
  - `internal/domain/leave/store_data.go`
  - `internal/transport/http/handlers/leave/handlers.go`
  - `frontend/src/features/leave/pages/Leave.jsx`
  - `frontend/src/features/leave/components/LeaveRequestsCard.jsx`
  - new migration for leave request documents.
- Schema already has:
  - `leave_requests.start_half`, `leave_requests.end_half`
  - `leave_types.requires_doc`
- Add missing bridge:
  - Request payload fields and persistence usage.
  - Document upload endpoint and secure storage strategy.
- Recommended doc storage:
  - Store file metadata in DB and encrypted file at rest.

### E3. Acceptance criteria
- HR policy requiring documentation is enforced.
- Half-day requests compute days correctly and update balances correctly.
- UI allows users/managers/HR to submit and review half-day/doc-based requests.

### E4. Test checklist
- [x] Unit: day calculation with full-day/half-day permutations.
- [x] Integration: create request with/without required document.
- [x] Integration: approval/rejection balance transitions with half-day.
- [x] Frontend tests for request form behavior.
- Note: backend leave integration tests are DB-backed and run when `TEST_DATABASE_URL` is configured.

---

## F. Reliable operations monitoring endpoints
Status (2026-02-08): Complete (query fix, filters, detail endpoint, pagination metadata, UI and tests implemented)

### F1. Checklist
- [x] Fix job-runs query parameter indexing bug.
- [x] Ensure consistent pagination metadata (`X-Total-Count`) for job-runs list.
- [x] Add filters for job status and date range.
- [x] Add endpoint for single job-run detail (including failure reason/details).
- [x] Add tests for filtered/unfiltered queries and pagination.
- [x] Update reports UI to show status/details reliably.

### F2. Implementation guidance
- Primary files:
  - `internal/domain/reports/store.go`
  - `internal/transport/http/handlers/reports/handlers.go`
  - `frontend/src/features/reports/pages/Reports.jsx`
- Immediate fix:
  - Correct SQL placeholder construction in `ListJobRuns` for both:
    - no `jobType` filter
    - with `jobType` filter
- Recommended enhancements:
  - Add total count query for job runs.
  - Add `status`, `startedFrom`, `startedTo` filters.
  - Return structured `details` shape in API response.

### F3. Acceptance criteria
- `/reports/jobs` works reliably with and without filters.
- UI job run table loads consistently and shows failed/completed context.
- Operators can identify failed runs and relevant details without DB access.

### F4. Test checklist
- [x] Unit/integration tests for query variants and pagination.
- [x] UI test for jobs filter and table rendering.

---

## 4) Definition of done for this initiative
- [x] All six items implemented and ready to merge.
- [x] Test coverage added for critical paths and failure modes.
- [x] `docs/API.md` updated for any changed payloads/endpoints.
- [x] `docs/DEPLOYMENT.md` updated for new environment variables.
- [x] Security and data-protection review completed for new flows.
- [x] Implementation log updated with completion dates and scope.

## 5) Release and rollback guidance
- Release strategy:
  - Deploy behind feature flags where behavior changes are user-visible.
  - Enable stricter rate limits gradually and observe false positives.
  - Roll out leave document enforcement with a transition period.
- Rollback strategy:
  - Keep additive DB migrations backward-compatible.
  - Preserve old payload fields during one release window.
  - Ensure idempotency changes fail closed (conflict) rather than duplicating side effects.
