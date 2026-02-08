# Deployment

## Docker (single container app + separate Postgres)

Build:

```
docker build -t hrm-app .
```

Run with Postgres (see docker-compose):

```
docker compose up --build
```

## Environment variables
- `DATABASE_URL` (required)
- `JWT_SECRET` (required)
- `DATA_ENCRYPTION_KEY` (required in production)
- `APP_ADDR` (default `:8080`)
- `FRONTEND_BASE_URL` (default `http://localhost:8080`)
- `FRONTEND_DIR` (default `frontend/dist`)
- `APP_ENV` (default `development`)
- `ALLOW_SELF_SIGNUP` (default `false`)
- `SEED_TENANT_NAME`
- `SEED_ADMIN_EMAIL`
- `SEED_ADMIN_PASSWORD`
- `SEED_SYSTEM_ADMIN_EMAIL`
- `SEED_SYSTEM_ADMIN_PASSWORD`
- `RUN_MIGRATIONS` (default `true`)
- `RUN_SEED` (default `true`)
- `MAX_BODY_BYTES` (default `1048576`)
- `RATE_LIMIT_PER_MINUTE` (default `60`)
- `LEAVE_ACCRUAL_INTERVAL` (default `24h`)
- `RETENTION_INTERVAL` (default `24h`)
- `PASSWORD_RESET_TTL` (default `2h`)
- `METRICS_ENABLED` (default `true`)
- `EMAIL_ENABLED` (default `false`)
- `EMAIL_FROM` (default `no-reply@example.com`)
- `SMTP_HOST`
- `SMTP_PORT` (default `587`)
- `SMTP_USER`
- `SMTP_PASSWORD`
- `SMTP_USE_TLS` (default `true`)

### Notes for immediate-priority controls
- No additional environment variables were introduced for the final hardening pass.
- Sensitive mutation endpoint throttling now uses `RATE_LIMIT_PER_MINUTE` as the base value and applies stricter per-endpoint windows internally (auth routes are throttled by both IP and email; authenticated sensitive mutations are throttled by user with IP fallback).

## Kubernetes
Use `deployments/k8s` for baseline manifests. Postgres should be deployed as a separate stateful service.
