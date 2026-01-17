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
- `APP_ADDR` (default `:8080`)
- `FRONTEND_DIR` (default `frontend/dist`)
- `SEED_TENANT_NAME`
- `SEED_ADMIN_EMAIL`
- `SEED_ADMIN_PASSWORD`
- `SEED_SYSTEM_ADMIN_EMAIL`
- `SEED_SYSTEM_ADMIN_PASSWORD`

## Kubernetes
Use `deployments/k8s` for baseline manifests. Postgres should be deployed as a separate stateful service.
