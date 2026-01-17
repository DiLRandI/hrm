# SME HR System

A full-stack HR system for SMEs covering Leave, Payroll, and Performance with GDPR tooling. Go API + React SPA, deployed as a single container with a separate PostgreSQL instance.

## Quick start (docker-compose)

```
make dev
```

Default login:
- Email: `admin@example.com`
- Password: `ChangeMe123!`

## Local dev

Backend:
```
export DATABASE_URL=postgres://hrm:hrm@localhost:5432/hrm?sslmode=disable
export JWT_SECRET=dev-secret

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
