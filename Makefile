.PHONY: help build run test clean docker-up docker-down migrate-up migrate-down

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod

# Service directories
SERVICES=api-gateway auth-service user-service staff-service inventory-service

# Output directory
OUT_DIR=./bin

help: ## Display this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

## Development

build: ## Build all services
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		$(GOBUILD) -o $(OUT_DIR)/$$service ./cmd/$$service; \
	done

build-%: ## Build a specific service (e.g., make build-auth-service)
	@echo "Building $*..."
	@$(GOBUILD) -o $(OUT_DIR)/$* ./cmd/$*

run-%: ## Run a specific service (e.g., make run-auth-service)
	@echo "Running $*..."
	@$(GOCMD) run ./cmd/$*

tidy: ## Run go mod tidy
	@$(GOMOD) tidy

test: ## Run tests
	@$(GOTEST) -v ./...

clean: ## Clean build artifacts
	@$(GOCLEAN)
	@rm -rf $(OUT_DIR)

## Docker

docker-up: ## Start all services with docker compose
	@cd deployments && docker compose up -d

docker-down: ## Stop all services
	@cd deployments && docker compose down

docker-logs: ## View logs from all services
	@cd deployments && docker compose logs -f

docker-logs-%: ## View logs from a specific service (e.g., make docker-logs-auth-service)
	@cd deployments && docker compose logs -f $*

docker-build: ## Build Docker images
	@cd deployments && docker compose build

docker-rebuild: ## Rebuild and restart all services
	@cd deployments && docker compose up -d --build

## Infrastructure only (databases + RabbitMQ)

infra-up: ## Start only infrastructure (databases, RabbitMQ)
	@cd deployments && docker compose up -d postgres-auth postgres-users postgres-staff postgres-inventory rabbitmq

infra-down: ## Stop infrastructure
	@cd deployments && docker compose stop postgres-auth postgres-users postgres-staff postgres-inventory rabbitmq

## Migrations

migrate-up: ## Run all migrations
	@echo "Running auth migrations..."
	@migrate -path migrations/auth -database "postgres://medflow:devpassword@localhost:5433/medflow_auth?sslmode=disable" up
	@echo "Running user migrations..."
	@migrate -path migrations/user -database "postgres://medflow:devpassword@localhost:5434/medflow_users?sslmode=disable" up
	@echo "Running staff migrations..."
	@migrate -path migrations/staff -database "postgres://medflow:devpassword@localhost:5435/medflow_staff?sslmode=disable" up
	@echo "Running inventory migrations..."
	@migrate -path migrations/inventory -database "postgres://medflow:devpassword@localhost:5436/medflow_inventory?sslmode=disable" up

migrate-down: ## Rollback all migrations
	@echo "Rolling back inventory migrations..."
	@migrate -path migrations/inventory -database "postgres://medflow:devpassword@localhost:5436/medflow_inventory?sslmode=disable" down -all
	@echo "Rolling back staff migrations..."
	@migrate -path migrations/staff -database "postgres://medflow:devpassword@localhost:5435/medflow_staff?sslmode=disable" down -all
	@echo "Rolling back user migrations..."
	@migrate -path migrations/user -database "postgres://medflow:devpassword@localhost:5434/medflow_users?sslmode=disable" down -all
	@echo "Rolling back auth migrations..."
	@migrate -path migrations/auth -database "postgres://medflow:devpassword@localhost:5433/medflow_auth?sslmode=disable" down -all

migrate-up-%: ## Run migrations for a specific service (e.g., make migrate-up-auth)
	@echo "Running $* migrations..."
	@migrate -path migrations/$* -database "postgres://medflow:devpassword@localhost:$(shell case $* in auth) echo 5433;; user) echo 5434;; staff) echo 5435;; inventory) echo 5436;; esac)/medflow_$*?sslmode=disable" up

migrate-down-%: ## Rollback migrations for a specific service
	@migrate -path migrations/$* -database "postgres://medflow:devpassword@localhost:$(shell case $* in auth) echo 5433;; user) echo 5434;; staff) echo 5435;; inventory) echo 5436;; esac)/medflow_$*?sslmode=disable" down 1

## Bridge Model: Tenant Schema Migrations (per service)

# User service tenant migrations
migrate-user-tenant-up: ## Create tenant schema in user DB (Usage: TENANT=test_practice)
	@if [ -z "$(TENANT)" ]; then echo "Error: TENANT not specified. Usage: make migrate-user-tenant-up TENANT=test_practice"; exit 1; fi
	@echo "Creating tenant schema in user service for tenant_$(TENANT)..."
	@docker exec -i medflow-db-users psql -U medflow -d medflow_users -c "CREATE SCHEMA IF NOT EXISTS tenant_$(TENANT);"
	@~/go/bin/migrate -path migrations/user/tenant -database "postgresql://medflow:devpassword@localhost:5434/medflow_users?sslmode=disable&search_path=tenant_$(TENANT)" up

# Staff service tenant migrations
migrate-staff-tenant-up: ## Create tenant schema in staff DB (Usage: TENANT=test_practice)
	@if [ -z "$(TENANT)" ]; then echo "Error: TENANT not specified. Usage: make migrate-staff-tenant-up TENANT=test_practice"; exit 1; fi
	@echo "Creating tenant schema in staff service for tenant_$(TENANT)..."
	@docker exec -i medflow-db-staff psql -U medflow -d medflow_staff -c "CREATE SCHEMA IF NOT EXISTS tenant_$(TENANT);"
	@~/go/bin/migrate -path migrations/staff/tenant -database "postgresql://medflow:devpassword@localhost:5435/medflow_staff?sslmode=disable&search_path=tenant_$(TENANT)" up

# Inventory service tenant migrations
migrate-inventory-tenant-up: ## Create tenant schema in inventory DB (Usage: TENANT=test_practice)
	@if [ -z "$(TENANT)" ]; then echo "Error: TENANT not specified. Usage: make migrate-inventory-tenant-up TENANT=test_practice"; exit 1; fi
	@echo "Creating tenant schema in inventory service for tenant_$(TENANT)..."
	@docker exec -i medflow-db-inventory psql -U medflow -d medflow_inventory -c "CREATE SCHEMA IF NOT EXISTS tenant_$(TENANT);"
	@~/go/bin/migrate -path migrations/inventory/tenant -database "postgresql://medflow:devpassword@localhost:5436/medflow_inventory?sslmode=disable&search_path=tenant_$(TENANT)" up

# Create tenant across ALL services
create-tenant: ## Create tenant across all service databases (Usage: TENANT=test_practice)
	@if [ -z "$(TENANT)" ]; then echo "Error: TENANT not specified. Usage: make create-tenant TENANT=test_practice"; exit 1; fi
	@echo "Creating tenant_$(TENANT) across all services..."
	@make migrate-user-tenant-up TENANT=$(TENANT)
	@make migrate-staff-tenant-up TENANT=$(TENANT)
	@make migrate-inventory-tenant-up TENANT=$(TENANT)
	@echo "Tenant tenant_$(TENANT) created successfully across all services!"

## Development Workflow

dev: infra-up ## Start infrastructure and run all services locally
	@echo "Infrastructure started. Run services with:"
	@echo "  make run-api-gateway"
	@echo "  make run-auth-service"
	@echo "  make run-user-service"
	@echo "  make run-staff-service"
	@echo "  make run-inventory-service"

seed: ## Seed the database with test data
	@echo "Seeding database..."
	@go run ./scripts/seed/main.go

## Code Quality

lint: ## Run linter
	@golangci-lint run ./...

fmt: ## Format code
	@gofmt -s -w .

vet: ## Run go vet
	@$(GOCMD) vet ./...

## Install tools

tools: ## Install development tools
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
