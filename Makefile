APP_NAME=hrm
BIN_DIR=bin
FRONTEND_DIR=frontend
DATABASE_URL?=postgres://hrm:hrm@localhost:5432/hrm?sslmode=disable
JWT_SECRET?=dev-secret
DATA_ENCRYPTION_KEY?=1QFualBeEVX7XW3hmeBPGaQQD255ctbtvnKXJHakYjo=
APP_ADDR?=:8080
SEED_TENANT_NAME?=Default Tenant
SEED_ADMIN_EMAIL?=admin@example.com
SEED_ADMIN_PASSWORD?=ChangeMe123!
SEED_SYSTEM_ADMIN_EMAIL?=sysadmin@example.com
SEED_SYSTEM_ADMIN_PASSWORD?=ChangeMe123!
EMAIL_ENABLED?=false
RUN_MIGRATIONS?=true
RUN_SEED?=true

.PHONY: \
	dev \
	dev-local \
	dev-db \
	dev-backend \
	dev-frontend \
	build \
	build-backend \
	build-frontend \
	test \
	test-backend \
	test-frontend \
	docker-build \
	docker-up \
	fmt \
	clean

## Run the full stack in containers

dev: docker-up

dev-db:
	docker compose up db

dev-backend:
	DATABASE_URL="$(DATABASE_URL)" \
	JWT_SECRET="$(JWT_SECRET)" \
	DATA_ENCRYPTION_KEY="$(DATA_ENCRYPTION_KEY)" \
	APP_ADDR="$(APP_ADDR)" \
	SEED_TENANT_NAME="$(SEED_TENANT_NAME)" \
	SEED_ADMIN_EMAIL="$(SEED_ADMIN_EMAIL)" \
	SEED_ADMIN_PASSWORD="$(SEED_ADMIN_PASSWORD)" \
	SEED_SYSTEM_ADMIN_EMAIL="$(SEED_SYSTEM_ADMIN_EMAIL)" \
	SEED_SYSTEM_ADMIN_PASSWORD="$(SEED_SYSTEM_ADMIN_PASSWORD)" \
	EMAIL_ENABLED="$(EMAIL_ENABLED)" \
	RUN_MIGRATIONS="$(RUN_MIGRATIONS)" \
	RUN_SEED="$(RUN_SEED)" \
	go run ./cmd/server


dev-frontend:
	cd $(FRONTEND_DIR) && npm install && npm run dev

build: build-backend build-frontend

build-backend:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/server

build-frontend:
	cd $(FRONTEND_DIR) && npm install && npm run build

test: test-backend test-frontend

test-backend:
	go test ./...

test-frontend:
	cd $(FRONTEND_DIR) && npm install && npm run test

docker-build:
	docker build -t hrm-app .

docker-up:
	docker compose up --build

clean:
	rm -rf $(BIN_DIR)
