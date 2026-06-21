
APP_NAME    := taskmgr
BIN_DIR     := bin
APP_BIN     := $(BIN_DIR)/app
MIGRATE_BIN := $(BIN_DIR)/migrate

GO          ?= go
DOCKER      ?= docker
COMPOSE     ?= docker compose

COVERAGE    := -coverprofile=coverage.out -covermode=atomic

.PHONY: help tidy build run test test-integration cover up down down-v logs migrate-create

help:
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

tidy:
	$(GO) mod tidy

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(APP_BIN)     ./cmd/app
	$(GO) build -o $(MIGRATE_BIN) ./cmd/migrate

run:
	$(GO) run ./cmd/app

test:
	$(GO) test $(COVERAGE) ./internal/... ./pkg/...

test-integration:
	INTEGRATION=1 $(GO) test -tags=integration ./test/integration/...

cover:
	$(GO) tool cover -html=coverage.out

up: ## Поднять docker-compose стек
	$(COMPOSE) up -d --build

down: ## Остановить docker-compose стек
	$(COMPOSE) down

down-v:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f

migrate-create:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-create name=<name>"; exit 1; fi
	@timestamp=$$(date +%Y%m%d%H%M%S); \
	 touch migrations/$${timestamp}_$(name).up.sql migrations/$${timestamp}_$(name).down.sql; \
	 echo "Created migrations/$${timestamp}_$(name).up.sql and .down.sql"
