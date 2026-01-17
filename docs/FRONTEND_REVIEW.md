# Frontend Review (Lead React Engineer)

## Scope
React SPA under `frontend/src`. Review focuses on architecture, data flow, UX consistency, and test coverage.

## Architecture Snapshot
- Feature-based layout under `frontend/src/features/*` with simple page-level state management.
- Shared API client in `frontend/src/services/apiClient.js` (JSON + download support).
- Auth context provides token storage and user identity.

## Strengths
- Clear module separation (core, leave, payroll, performance, GDPR, reports).
- Lightweight fetch wrapper with consistent error handling.
- Role-aware rendering for HR/manager actions in core workflows.
- Added pagination controls for high-volume views (employees, leave requests, audit, DSAR/access logs).
- Expanded test coverage for key workflow pages plus Playwright smoke E2E.

## Findings (Key Gaps)
### Data Fetching & State
- Ad-hoc data fetching per page; no caching or shared loading/error states.
- Multiple parallel requests are managed manually; repeated patterns could be standardized.
- No optimistic updates or background refresh for time-sensitive dashboards.

### UX & Validation
- Form validation is minimal and mostly server-driven; JSON inputs (templates, PIPs) rely on user correctness.
- Error states are inconsistent and not centrally surfaced (no toast/alert system).
- Large tables now include pagination for high-volume modules; other lists still lack filtering/search.

### Access Control & Routing
- Role gating is done within pages but there are no route-level guards for HR/manager-only screens.
- Employee IDs are entered manually in several forms; a directory picker would reduce errors.

### Testing
- Unit tests cover auth, dashboard, leave, payroll, and GDPR flows; performance and notifications still have light coverage.
- Playwright smoke E2E exists and can be run against a live environment.

## Recommendations (Prioritized)
1. Introduce a data-fetching layer (TanStack Query) for caching, loading states, and error handling.
2. Add shared form components and schema validation (Zod) with inline error messaging.
3. Implement pagination/search/filtering for lists and tables.
4. Add route-level role guards and a centralized notification/toast system.
5. Expand tests: unit tests for key pages + Playwright E2E for leave, payroll, performance, GDPR.

## UI Consistency
- Layout classes are consistent; a small component library (Buttons, Tables, Cards, Modals) will reduce duplication and improve accessibility.
