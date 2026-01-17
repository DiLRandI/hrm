# Frontend Review (Lead React Engineer)

## Architecture Snapshot
- Feature-based layout under `frontend/src/features/*`.
- Shared API client in `frontend/src/services/apiClient.js`.
- Auth context provides token storage and user identity.

## Strengths
- Clear separation by feature modules (core, leave, payroll, performance, GDPR, reports).
- Lightweight fetch wrapper with consistent error handling.
- Role-aware rendering for HR/manager actions in key pages.

## Risks & Gaps
- **Data fetching** is ad-hoc per page; no caching or shared query state.
- **Form validation** is minimal and mostly server-driven; UX errors are not standardized.
- **State coupling**: multiple pages manage large state sets locally; could benefit from shared hooks.
- **Testing**: only basic tests exist (auth + dashboard); new workflows are untested.

## Recommendations
1. Adopt a data-fetching layer (TanStack Query) to standardize caching, loading, and error states.
2. Centralize form validation and errors (Zod + shared form components).
3. Add UI-level role guards (route-level) and better empty/failed states.
4. Expand unit tests and add Playwright E2E flows for leave, payroll, performance, and GDPR.

## UI Consistency
- Current pages use consistent layout classes; a small component library (buttons, tables, cards) would reduce duplication.
