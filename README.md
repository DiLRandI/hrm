# SME HR System

A full-stack HR system for SMEs covering Leave, Payroll, and Performance with GDPR tooling. Go API + React SPA, deployed as a single container with a separate PostgreSQL instance.

## Quick start (docker-compose)

```
make dev
```

Default login is created from `SEED_ADMIN_EMAIL` and `SEED_ADMIN_PASSWORD` if `RUN_SEED=true`.

## Local dev

Backend:
```
export DATABASE_URL=postgres://hrm:hrm@localhost:5432/hrm?sslmode=disable
export JWT_SECRET=dev-secret
export SEED_ADMIN_EMAIL=admin@example.com
export SEED_ADMIN_PASSWORD=ChangeMe123!

go run ./cmd/server
```

Frontend:
```
cd frontend
npm install
npm run dev
```

## Tests

```
make test
```

## Documentation
- `docs/ARCHITECTURE.md`
- `docs/API.md`
- `docs/DEPLOYMENT.md`
- `docs/IMPLEMENTATION_LOG.md`
