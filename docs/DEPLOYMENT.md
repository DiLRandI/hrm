# Deployment

## Container Build and Run

Build image:

```sh
docker build -t hrm-app .
```

Run local stack with Docker Compose:

```sh
docker compose up --build
```

## Runtime Environment Variables

### Required/critical
- `DATABASE_URL`: required in all environments.
- `JWT_SECRET`: strongly recommended always, required for production.
- `DATA_ENCRYPTION_KEY`: required in production; also required if you want MFA/encryption-backed fields in non-production.

### Network and runtime
- `APP_ADDR` (default `:8080`)
- `APP_ENV` (default `development`)
- `FRONTEND_DIR` (default `frontend/dist`)
- `FRONTEND_BASE_URL` (default `http://localhost:8080`) for reset-link generation

### Bootstrapping and startup
- `RUN_MIGRATIONS` (default `true`)
- `RUN_SEED` (default `true`)
- `SEED_TENANT_NAME` (default `Default Tenant`)
- `SEED_SYSTEM_ADMIN_EMAIL` (required for fresh bootstrap when no `SystemAdmin` exists yet)
- `SEED_SYSTEM_ADMIN_PASSWORD` (required for fresh bootstrap when no `SystemAdmin` exists yet)
- `SEED_ADMIN_EMAIL` (optional initial `HR` user)
- `SEED_ADMIN_PASSWORD` (optional initial `HR` user)

### API protection and jobs
- `MAX_BODY_BYTES` (default `1048576`)
- `RATE_LIMIT_PER_MINUTE` (default `60`)
- `LEAVE_ACCRUAL_INTERVAL` (default `24h`)
- `RETENTION_INTERVAL` (default `24h`)
- `PASSWORD_RESET_TTL` (default `2h`)
- `METRICS_ENABLED` (default `true`)

### Email/notification settings
- `EMAIL_ENABLED` (default `false`)
- `EMAIL_FROM` (default `no-reply@example.com`)
- `SMTP_HOST` (required when `EMAIL_ENABLED=true`)
- `SMTP_PORT` (default `587`)
- `SMTP_USER`
- `SMTP_PASSWORD`
- `SMTP_USE_TLS` (default `true`)

### Reserved/not currently active in behavior
- `ALLOW_SELF_SIGNUP` is loaded by config but currently not used by any active auth/signup route.

## Role Provisioning Model
- Bootstrap seed ensures role catalog: `SystemAdmin`, `HR`, `HRManager`, `Manager`, `Employee`.
- Ongoing user provisioning is API-driven (`/api/v1/users`) with role-creation policy checks.

## Kubernetes Notes

`deployments/k8s` provides baseline manifests, not production-ready manifests:
- `deployments/k8s/postgres.yaml` uses `emptyDir`, so DB data is ephemeral.
- `deployments/k8s/hrm-deployment.yaml` includes only a minimal env set; add production env vars/secrets before use.
- configure readiness/liveness probes, persistent volumes, and secret management before production rollout.
