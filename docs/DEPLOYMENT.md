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
- `METRICS_ENABLED` (default `true`)
- `EMAIL_ENABLED` (default `false`)
- `EMAIL_FROM` (default `no-reply@example.com`)
- `SMTP_HOST`
- `SMTP_PORT` (default `587`)
- `SMTP_USER`
- `SMTP_PASSWORD`
- `SMTP_USE_TLS` (default `true`)

## Kubernetes
Use `deployments/k8s` for baseline manifests. Postgres should be deployed as a separate stateful service.
