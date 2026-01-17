# Deadcode Review & Plan

Command:
- `deadcode ./...`

Findings:
- `internal/transport/http/handlers/auth/handlers.go:43:19` — `Handler.RegisterRoutes` is unreachable (never called).

Why it is unused:
- The router wires auth endpoints directly with `HandleLogin`, `HandleLogout`, etc., so `RegisterRoutes` is unused scaffolding.

Plan:
1. Remove `RegisterRoutes` from `internal/transport/http/handlers/auth/handlers.go` to reduce dead code.
2. If a central registration pattern is desired, switch router wiring to use `RegisterRoutes` consistently across handlers.

Follow-up:
- Re-run `deadcode ./...` after cleanup to ensure no new unused functions are introduced.
