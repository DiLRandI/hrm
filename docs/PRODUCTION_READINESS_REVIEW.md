# Production Readiness & Security Review

Date: 2026-01-17
Scope: Go backend (`cmd/`, `internal/`), React frontend (`frontend/`), Postgres schema (`migrations/`).

## Executive Summary
This codebase shows a clear domain-based structure and a consistent API envelope, but it is not production-ready yet. There are multiple security and operational gaps (authorization, secrets, session management, rate limiting, input validation, and transport hardening) that fall short of current industry baselines (OWASP ASVS, OWASP API Security Top 10, and NIST guidance). The most urgent risks are broken object-level authorization, insufficient role enforcement, and weak secret/config defaults.

## Architecture & Code Patterns Observed

### Backend (Go)
- Entry point: `cmd/server/main.go` -> `internal/app/server` uses chi router and middleware.
- Domain logic is grouped under `internal/domain/*` with constants and some tests.
- HTTP handlers are in `internal/transport/http/handlers/*` and use a shared `api` response envelope.
- Mixed responsibility: many handlers execute SQL directly instead of delegating to domain stores/services.
- Auth uses JWT (HS256) with request context injection via middleware.

### Frontend (React)
- Feature-oriented structure under `frontend/src/features/<area>/pages`.
- API access via `frontend/src/services/apiClient.js` (fetch wrapper with JSON + Bearer token).
- Auth state handled via context (`AuthProvider`), with route guard in `App.jsx`.
- UI state and data fetching are local to pages (no shared query/cache layer).

## Production Readiness Verdict
**Not production-ready.** The current implementation lacks several security controls and operational safeguards expected in production. The most significant gaps are authorization enforcement, secret/config hardening, and API abuse protections.

## Security Findings (Highest Priority First)

### P0 (Critical)
1) **Broken Object-Level Authorization (BOLA)**
   - Several endpoints accept IDs or query parameters without verifying ownership or role-scoped access.
   - Examples:
     - Leave balances can be queried with arbitrary `employeeId`.
     - Performance goals and review tasks list across the whole tenant.
     - GDPR DSAR listing/requesting is not restricted to HR/admin roles.
   - Impact: data exfiltration across employees within a tenant (OWASP API Security Top 10).

2) **Role Enforcement Gaps**
   - Some endpoints intended for HR/manager use are not guarded by role checks (e.g., reports dashboards, DSAR list).
   - A permission middleware exists but is not used.

3) **Default Secrets and Seeded Admin Credentials**
   - `JWT_SECRET` defaults to `change-me`, seed admin password defaults to `ChangeMe123!`.
   - In production this is a direct compromise risk if envs are misconfigured.

### P1 (High)
4) **Session Management Weaknesses**
   - JWTs are short-lived but there is no refresh rotation or server-side invalidation.
   - Logout is a no-op; sessions table exists but is unused.

5) **Password Reset Tokens Stored in Plaintext**
   - Reset tokens are stored raw and never hashed; a DB read leak exposes reset tokens.

6) **Missing Rate Limiting / Abuse Protection**
   - Login and password reset endpoints have no throttling or lockout.
   - No global request rate limiting for API.

7) **Sensitive Data at Rest**
   - Salary, bank account, national ID are stored in plaintext.
   - There is no enforced encryption-at-rest policy in code or migrations.

### P2 (Medium)
8) **Missing Security Headers / Transport Hardening**
   - No HSTS, CSP, X-Content-Type-Options, etc.
   - No TLS enforcement (assumes upstream proxy handles it).

9) **Input Validation & Payload Limits**
   - JSON payloads are decoded without size limits and minimal validation.
   - No schema validation, enum checks, or consistent input normalization.

10) **Insufficient Audit Trails**
    - There are access logs for employee profile reads, but not for critical changes (payroll runs, GDPR exports, role changes, data edits).

## Operational & Reliability Gaps
- No graceful shutdown / server timeouts (risk of slowloris or hanging connections).
- No structured tracing or metrics.
- Migrations and seeding run at startup (undesirable for production releases).
- No pagination for large list endpoints.
- Limited integration and API tests; no end-to-end coverage.

## Recommended Improvements (Prioritized)

### Security & Access Control
- Enforce authorization on every endpoint with explicit ownership/role checks.
- Replace role checks with permission middleware (use `RequirePermission`).
- Add scope checks for all employee-related resources (BOLA prevention).
- Hash password reset tokens before storage; enforce one-time use and rotation.
- Add login/reset rate limiting and account lockout thresholds.
- Enforce config validation at startup: refuse defaults for `JWT_SECRET`, admin seed passwords.
- Add MFA readiness (flags exist in DB but no implementation).
- Introduce audit logging for sensitive actions and data changes.

### Auth & Session Management
- Implement refresh tokens with rotation and revocation (or short-lived tokens + session store).
- Add server-side token invalidation for logout and user disable.

### Transport & API Hardening
- Add HTTP server timeouts (read, header, write, idle) and max header size.
- Add security headers (CSP, HSTS, X-Content-Type-Options, Referrer-Policy).
- Add CORS policy if frontends are served cross-origin.
- Set JSON body size limits for all handlers (middleware).

### Data Protection
- Encrypt sensitive fields at rest (DB-level or application-level).
- Consider field-level encryption for national ID, bank account, salary.
- Ensure DSAR exports are stored in secure object storage and access-checked.

### Code Structure
- Move direct SQL from handlers into domain stores/services.
- Create a consistent validation layer (e.g., `validator` package per domain).
- Add transactions for multi-step writes.

### Frontend
- Add role-based UI gating and route guards for privileged views.
- Add error boundaries and centralized toast/errors for API errors.
- Use a query/cache library (TanStack Query) for consistent data fetching.
- Consider moving JWT storage to httpOnly cookies (with CSRF protection) or in-memory storage to reduce XSS exposure.

### Testing & Tooling
- Add API integration tests for auth + authorization.
- Add security tests for BOLA and role enforcement.
- Add dependency scanning (govulncheck, npm audit) in CI.

## References (Best Practices)
- OWASP API Security Top 10 (2023)
- OWASP ASVS (latest stable)
- OWASP Session Management Cheat Sheet
- OWASP Password Storage Cheat Sheet
- OWASP Security Headers Cheat Sheet
- OWASP CSRF Prevention Cheat Sheet
- NIST SP 800-63B (Digital Identity Guidelines)
- Go net/http Server documentation (timeouts, limits)

## Suggested Next Steps
1) Fix P0/P1 security issues (authorization gaps, secrets, reset token storage, rate limiting).
2) Add server timeouts and security headers.
3) Implement refresh tokens or session invalidation.
4) Introduce validation and request size limits.
5) Add integration tests and CI security checks.
