# HR Requirements Gap Analysis (Current)

Date: 2026-02-22
Repository: `hrm`

## 1) Scope and method
This review compares implemented behavior in:
- backend routes and handlers (`internal/app/server/server.go`, `internal/transport/http/handlers/*`)
- domain/store logic (`internal/domain/*`)
- frontend feature surfaces (`frontend/src/features/*`, `frontend/src/app/App.jsx`)
- schema migrations (`migrations/*.sql`)

It separates closed gaps (already implemented) from active gaps (still requiring work).

## 2) Closed gaps (implemented)

| ID | Previously identified gap | Current status | Evidence |
|---|---|---|---|
| C-1 | Password reset delivery flow missing | Closed | Reset token generation + mail send + generic response in `internal/transport/http/handlers/auth/handlers.go` |
| C-2 | Transactional payroll finalization / hard idempotency missing | Closed | Finalize flow requires `Idempotency-Key` and enforces conflict-safe behavior in `internal/transport/http/handlers/payroll/handlers.go` |
| C-3 | Broader sensitive mutation abuse protection missing | Closed | Sensitive mutation rate limiting middleware applied in `internal/app/server/server.go` and `internal/transport/http/middleware/ratelimit.go` |
| C-4 | Structured payload validation inconsistent | Closed | Shared validation usage across auth/leave/payroll/gdpr handlers in `internal/transport/http/shared/validation.go` and handler files |
| C-5 | Leave half-day/doc workflow incomplete | Closed | `startHalf`/`endHalf` + multipart document support in `internal/transport/http/handlers/leave/handlers.go` with migration `migrations/0007_leave_request_documents.sql` |
| C-6 | Job-runs monitoring query reliability issue | Closed | Dynamic filter-safe query building in `internal/domain/reports/store.go` |

## 3) Active gaps (prioritized)

### P0: High-priority correctness/security

| ID | Active gap | Current observation | Impact |
|---|---|---|---|
| P0-1 | Manager-history tenant/role scoping hardening | `GET /employees/{employeeID}/manager-history` fetch path currently calls `ManagerHistory` with only `employeeID`; store query is not tenant-bound and handler does not apply self/manager visibility checks (`internal/transport/http/handlers/core/handlers.go`, `internal/domain/core/store.go`) | Potential privacy data exposure across unintended employee records |

### P1: Functional breadth (not yet implemented)

| ID | Active gap | Current observation |
|---|---|---|
| P1-1 | Recruiting / ATS | No recruiting domain/routes/UI/schema |
| P1-2 | Onboarding workflow | No onboarding checklist/task automation module |
| P1-3 | Offboarding workflow | No offboarding case/checklist/asset-return module |
| P1-4 | Time & attendance | No timesheets/clocking/overtime engine |
| P1-5 | Benefits administration | No plans/enrollment/dependents workflow |
| P1-6 | Compensation planning | No review-cycle compensation planning workflow |
| P1-7 | General employee document + e-signature lifecycle | No broad document/e-sign module beyond specific GDPR/leave/payroll artifacts |
| P1-8 | Statutory payroll/tax compliance layer | Payroll model remains generic and not jurisdiction-specific |

### P2: Scale and ecosystem maturity

| ID | Active gap | Current observation |
|---|---|---|
| P2-1 | Integration ecosystem | No webhook/integration framework for external HR ecosystem systems |
| P2-2 | Multi-tenant SaaS model | Product remains single-tenant per deployment |
| P2-3 | Deep E2E coverage for critical workflows | E2E remains smoke-level; deeper multi-role payroll/compliance journeys are limited |

## 4) Recommended next implementation wave
1. Close P0-1 manager-history scoping (tenant-bound query + access policy checks + regression tests).
2. Expand E2E depth for payroll, leave approvals, GDPR exports, and notifications.
3. Select one P1 capability group (time & attendance or onboarding/offboarding) for next roadmap slice.

## 5) Source of truth docs
- API: `docs/API.md`
- Deployment/runtime config: `docs/DEPLOYMENT.md`
- Historical implementation timeline: `docs/IMPLEMENTATION_LOG.md`
