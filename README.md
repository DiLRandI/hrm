# SME HR System

Full-stack HR system for SMEs: Go API + React SPA, with PostgreSQL as the data store.

## Quick Start (Docker Compose)

```sh
make dev
```

Default seeded users (when `RUN_SEED=true`):
- HR admin: `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD`
- System admin: `SEED_SYSTEM_ADMIN_EMAIL` / `SEED_SYSTEM_ADMIN_PASSWORD`

## Local Development

Backend:
```sh
make dev-db
make dev-backend
```

Override env values when needed:
```sh
DATABASE_URL='postgres://hrm:hrm@localhost:5432/hrm?sslmode=disable' \
JWT_SECRET='dev-secret' \
DATA_ENCRYPTION_KEY='1QFualBeEVX7XW3hmeBPGaQQD255ctbtvnKXJHakYjo=' \
make dev-backend
```

Frontend:
```sh
make dev-frontend
```

## Tests

Unit/integration (backend + frontend):
```sh
make test
```

Backend DB-backed integration tests:
```sh
TEST_DATABASE_URL='postgres://hrm:hrm@localhost:5432/hrm_test?sslmode=disable' go test ./...
```

E2E smoke tests:
```sh
cd frontend && E2E_BASE_URL='http://localhost:8080' npm run test:e2e
```

## Documentation
- `docs/ARCHITECTURE.md`
- `docs/API.md`
- `docs/DEPLOYMENT.md`
- `docs/IMPLEMENTATION_LOG.md`
- `docs/HR_REQUIREMENTS_GAP_ANALYSIS.md`
