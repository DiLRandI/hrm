APP_NAME=hrm
BIN_DIR=bin
FRONTEND_DIR=frontend

.PHONY: dev dev-local dev-backend dev-frontend build build-backend build-frontend test test-backend test-frontend docker-build docker-up fmt clean

## Run the full stack in containers

dev: docker-up

dev-local:
	@echo "Run locally in two terminals:"
	@echo "  1) make dev-backend"
	@echo "  2) make dev-frontend"


dev-backend:
	DATABASE_URL?=postgres://hrm:hrm@localhost:5432/hrm?sslmode=disable \
	JWT_SECRET?=dev-secret \
	APP_ADDR?=:8080 \
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

fmt:
	gofmt -w cmd internal

clean:
	rm -rf $(BIN_DIR)
