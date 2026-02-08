# HR Requirements Gap Analysis (HR Manager View)

Date: 2026-02-08
Repository: `hrm`

## 1) Scope and method
This review compared:
- Implemented backend modules/routes under `internal/domain` and `internal/transport/http/handlers`
- Implemented frontend capability under `frontend/src/features`
- Data model in `migrations/*.sql`
- Existing product docs in `docs/API.md` and `docs/ARCHITECTURE.md`
- Baseline capability expectations from mainstream HR suites (BambooHR, Workday, ADP, Rippling, Oracle HCM)

## 2) What is already covered well
The application already covers a strong HR MVP baseline:
- Core employee data, departments, org chart, manager history, role/permission management
- Leave types/policies/balances/requests/approvals/calendar/reports/accrual runs
- Payroll schedules/groups/elements/period runs/finalization/payslips/journal/register exports
- Performance goals/reviews/feedback/check-ins/PIPs/summary
- GDPR DSAR, retention policies/runs, anonymization, consent, access logs
- Notifications, audit logs, role dashboards, and background job runs

Evidence:
- Backend routes: `internal/app/server/server.go`, `internal/transport/http/handlers/*`
- Frontend features: `frontend/src/features/*`
- Schema: `migrations/0001_init.sql`, `migrations/0002_leave_accruals.sql`, `migrations/0003_retention_runs.sql`, `migrations/0004_security_org_jobs.sql`, `migrations/0005_profile_contacts.sql`, `migrations/0006_departments_code.sql`

## 3) Missing requirements (prioritized)

### P0: Must-have before wider production rollout

| ID | Missing requirement | Gap observed in codebase | Why it matters |
|---|---|---|---|
| P0-1 | Complete password-reset delivery flow | `POST /auth/request-reset` stores token hash but does not send reset link/token to user. No auth-mail integration in auth handler flow (`internal/transport/http/handlers/auth/handlers.go`). | Users cannot practically recover accounts without DB/operator support. |
| P0-2 | Strong tenant/role scoping for manager-history access | Manager history handler does not enforce employee self/manager scope; store query is not tenant-filtered (`internal/transport/http/handlers/core/handlers.go`, `internal/domain/core/store.go`). | Privacy and data-leak risk for employee reporting-line history. |
| P0-3 | Transactional payroll finalization and hard idempotency for critical compliance actions | Finalization/payslip generation are multi-step operations without one DB transaction; project log already flags this as pending (`docs/IMPLEMENTATION_LOG.md`). | Prevents partial finalize states and reconciliation risk. |
| P0-4 | Broader abuse protection on sensitive mutation endpoints | Rate limit currently applied mainly to login/reset endpoints (`internal/app/server/server.go`), while GDPR/approval/import operations are high-impact. | Reduces brute-force/automation abuse and accidental overload. |
| P0-5 | Structured payload validation for enums/ranges/date logic | Current handlers rely mostly on ad hoc checks and project log marks structured validation as pending (`docs/IMPLEMENTATION_LOG.md`). | Prevents invalid HR/payroll records and downstream data quality issues. |
| P0-6 | Reliable operations monitoring endpoints | Job-run listing query in reports store uses positional placeholders that can fail for the unfiltered path (`internal/domain/reports/store.go`). | HR/ops teams need dependable visibility into accrual/retention/payroll job execution. |

### P1: Needed for competitive HR operations coverage

| ID | Missing requirement | Gap observed in codebase | Why it matters |
|---|---|---|---|
| P1-1 | Recruiting / ATS | No recruiting domain, routes, UI feature, or schema (no requisitions/candidates/interview tables in `migrations/*`; no `recruiting` feature folder). | Core HR systems typically include applicant tracking and hiring pipeline. |
| P1-2 | Onboarding workflow | No onboarding tasks/checklists/forms/provisioning workflow modules in backend/frontend. | New-hire readiness and compliance depend on structured onboarding. |
| P1-3 | Offboarding workflow | No termination checklist, asset return, revocation workflow, or offboarding case tracking module. | Controlled exits are required for compliance/security. |
| P1-4 | Time & attendance / time tracking | No timesheets, clock in/out, overtime, or attendance policy engine in data model or API. | Payroll accuracy and labor compliance typically require time tracking. |
| P1-5 | Benefits administration | No plan catalog, eligibility, enrollment, dependents, contribution modeling. | Benefits are a standard HRIS capability and key employee self-service need. |
| P1-6 | Compensation planning (beyond base salary field) | Only base salary and payroll adjustments exist; no comp review cycles/bands/merit workflows. | HR needs controlled compensation planning with approval and audit trails. |
| P1-7 | Employee document management + e-signature | No general document store for contracts/policies/signatures (outside payslip/DSAR files). | HR requires secure recordkeeping and policy acknowledgment evidence. |
| P1-8 | Leave workflow completeness | Schema has `start_half`/`end_half` and `requires_doc`, but request APIs/UI do not support half-day selection or document upload/enforcement (`migrations/0001_init.sql`, leave handlers/pages). | Leave policies are often half-day/document-dependent and compliance-sensitive. |
| P1-9 | Leave conflict and policy rule checks | Day calculation is simple inclusive date difference (`internal/domain/leave/logic.go`), with no overlap prevention/working-day policy engine. | Prevents duplicate/incorrect leave booking and balance errors. |
| P1-10 | Statutory payroll/tax compliance layer | Payroll calc is base+inputs-deductions (`internal/domain/payroll/calc.go`), without tax tables/statutory filings. | Required for real payroll operations beyond internal net-pay estimation. |

### P2: Strategic scale and maturity requirements

| ID | Missing requirement | Gap observed in codebase | Why it matters |
|---|---|---|---|
| P2-1 | Learning / development / skills / succession planning | No LMS, skills inventory, career mobility, succession module. | Important for talent retention and leadership pipeline. |
| P2-2 | Advanced workforce analytics/planning | Reports are lightweight KPI snapshots only (`internal/domain/reports/*`). | HR leadership usually needs headcount/attrition/forecast and scenario planning. |
| P2-3 | Integration ecosystem (identity/accounting/benefits/calendar/webhooks) | No dedicated integration/webhook framework. | Essential for operating HR stack in real environments. |
| P2-4 | Multi-tenant enterprise model | Architecture decision is single-tenant per deployment (`docs/IMPLEMENTATION_LOG.md`). | Limits SaaS-style scale and centralized operations. |
| P2-5 | End-to-end QA depth for critical workflows | E2E is smoke-only (`frontend/e2e/smoke.spec.js`); implementation log also calls out workflow test expansion. | Reduces release confidence for payroll/compliance-critical changes. |

## 4) Suggested requirement roadmap

### Wave 1 (P0)
- Complete account recovery delivery + harden access controls + transactional/idempotent compliance operations + validation/rate limiting.

### Wave 2 (P1)
- Add Workforce Operations module set: ATS, onboarding/offboarding, time tracking, benefits, compensation, document/signature management.

### Wave 3 (P2)
- Add strategic talent and planning capabilities, integration platform, and enterprise scale patterns.

## 5) External benchmark references used
- BambooHR platform capabilities: https://www.bamboohr.com/platform
- Workday absence management: https://www.workday.com/en-us/products/human-capital-management/absence-management.html
- Workday time tracking: https://www.workday.com/en-us/products/human-capital-management/time-tracking.html
- Workday employee experience/talent capabilities: https://www.workday.com/en-us/products/human-capital-management/employee-experience.html
- ADP HR capabilities overview: https://www.adp.com/what-we-offer/hr-and-payroll-software.aspx
- Rippling platform capabilities: https://www.rippling.com/platform
- Oracle HCM (core HR, talent, workforce mgmt): https://www.oracle.com/human-capital-management/
