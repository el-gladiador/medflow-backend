.PHONY: help build run test clean docker-up docker-down migrate-up migrate-down \
	cloud-setup cloud-build-all deploy-all cloud-urls cloud-submit-all model-upload

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

# Database configuration
# Single PostgreSQL database (RLS-based multi-tenancy)
# Supports both local Docker (medflow) and Supabase (postgres)
# Two roles:
#   medflow/postgres = superuser, used ONLY for migrations
#   medflow_app      = app role, used by services at runtime (RLS enforced)
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_NAME ?= medflow
DB_SSL_MODE ?= disable
DB_PASSWORD ?= devpassword

# Migration database URL (superuser - for schema changes only)
DB_MIGRATE_USER ?= medflow
MIGRATE_DATABASE_URL ?= postgres://$(DB_MIGRATE_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)

# Application database URL (non-superuser - RLS enforced)
DB_APP_USER ?= medflow_app
APP_DATABASE_URL ?= postgres://$(DB_APP_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)

# Determine if we're targeting a local Docker DB or a remote DB
# Used by tenant management commands to choose docker exec vs psql
IS_LOCAL_DB := $(if $(filter localhost 127.0.0.1,$(DB_HOST)),true,false)

# psql command for remote DB access
PSQL_REMOTE = psql "$(MIGRATE_DATABASE_URL)"

# psql command dispatcher: docker exec for local, psql for remote
define run_psql
	$(if $(filter true,$(IS_LOCAL_DB)), \
		docker exec -i medflow-db psql -U $(DB_MIGRATE_USER) -d $(DB_NAME) -c $(1), \
		$(PSQL_REMOTE) -c $(1))
endef

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

test-short: ## Run tests (skip integration tests)
	@$(GOTEST) -short -v ./...

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

## Infrastructure only (database + RabbitMQ)

infra-up: ## Start only infrastructure (database, RabbitMQ)
	@cd deployments && docker compose up -d postgres rabbitmq

infra-down: ## Stop infrastructure
	@cd deployments && docker compose stop postgres rabbitmq

## Migrations (single database, unified migrations)
## Uses superuser for schema changes.
## Migrations path: migrations/supabase/

migrate-up: ## Run all migrations
	@echo "Running migrations against $(DB_HOST):$(DB_PORT)/$(DB_NAME) as $(DB_MIGRATE_USER)..."
	@migrate -path migrations/supabase -database "$(MIGRATE_DATABASE_URL)" up

migrate-down: ## Rollback all migrations
	@echo "Rolling back all migrations..."
	@migrate -path migrations/supabase -database "$(MIGRATE_DATABASE_URL)" down -all

migrate-down-1: ## Rollback last migration
	@echo "Rolling back last migration..."
	@migrate -path migrations/supabase -database "$(MIGRATE_DATABASE_URL)" down 1

migrate-status: ## Show migration status
	@migrate -path migrations/supabase -database "$(MIGRATE_DATABASE_URL)" version

migrate-force-%: ## Force migration version (e.g., make migrate-force-8)
	@echo "Forcing migration version to $*..."
	@migrate -path migrations/supabase -database "$(MIGRATE_DATABASE_URL)" force $*

migrate-build: ## Build migration Docker image locally
	@echo "Building migration image..."
	@docker build -t medflow-migrate -f deployments/docker/Dockerfile.migrate .

migrate-docker: ## Run migrations via Docker against local postgres
	@echo "Running migrations via Docker..."
	@docker run --rm --network medflow-backend_medflow-network \
		-e DATABASE_URL="postgres://$(DB_MIGRATE_USER):$(DB_PASSWORD)@medflow-db:5432/$(DB_NAME)?sslmode=$(DB_SSL_MODE)" \
		medflow-migrate

## Tenant Management (RLS-based)
## Tenants are managed via INSERT into public.tenants, not schema creation.
## Works with both local Docker DB and remote Supabase DB.

create-tenant: ## Create a new tenant (Usage: make create-tenant TENANT_NAME="Praxis Mueller" TENANT_SLUG=praxis-mueller)
	@if [ -z "$(TENANT_NAME)" ] || [ -z "$(TENANT_SLUG)" ]; then \
		echo "Error: TENANT_NAME and TENANT_SLUG required."; \
		echo "Usage: make create-tenant TENANT_NAME=\"Praxis Mueller\" TENANT_SLUG=praxis-mueller"; \
		exit 1; \
	fi
	@echo "Creating tenant '$(TENANT_NAME)' ($(TENANT_SLUG)) on $(DB_HOST)/$(DB_NAME)..."
	@$(call run_psql,"INSERT INTO public.tenants (slug, name, contact_email, subscription_tier, subscription_status) \
		 VALUES ('$(TENANT_SLUG)', '$(TENANT_NAME)', '$(TENANT_SLUG)@medflow.de', 'standard', 'active') \
		 ON CONFLICT (slug) DO NOTHING;")
	@echo "Tenant created. Seed default roles with: make seed-tenant-roles TENANT_SLUG=$(TENANT_SLUG)"

seed-tenant-roles: ## Seed default roles for a tenant (Usage: make seed-tenant-roles TENANT_SLUG=praxis-mueller)
	@if [ -z "$(TENANT_SLUG)" ]; then \
		echo "Error: TENANT_SLUG required."; \
		exit 1; \
	fi
	@echo "Seeding roles for tenant $(TENANT_SLUG) on $(DB_HOST)/$(DB_NAME)..."
	@$(call run_psql,"INSERT INTO users.roles (tenant_id, name, display_name, display_name_de, description, is_system, permissions, level) \
		 SELECT t.id, r.name, r.display_name, r.display_name_de, r.description, true, r.permissions, r.level \
		 FROM public.tenants t, \
		 (VALUES \
			('admin', 'Admin', 'Administrator', 'Full system access', '[\"*\"]'::jsonb, 100), \
			('manager', 'Manager', 'Praxismanager', 'Staff and inventory management', '[\"staff.*\",\"inventory.*\",\"reports.*\",\"user.read\"]'::jsonb, 80), \
			('staff', 'Staff', 'Mitarbeiter', 'Basic access', '[\"inventory.read\",\"inventory.adjust\",\"profile.*\"]'::jsonb, 50) \
		 ) AS r(name, display_name, display_name_de, description, permissions, level) \
		 WHERE t.slug = '$(TENANT_SLUG)' \
		 ON CONFLICT (tenant_id, name) DO NOTHING;")
	@echo "Roles seeded for $(TENANT_SLUG)."

delete-tenant: ## Delete all tenant data (GDPR erasure) (Usage: make delete-tenant TENANT_SLUG=praxis-mueller)
	@if [ -z "$(TENANT_SLUG)" ]; then \
		echo "Error: TENANT_SLUG required."; \
		exit 1; \
	fi
	@echo "WARNING: This will permanently delete ALL data for tenant '$(TENANT_SLUG)' on $(DB_HOST)/$(DB_NAME)."
	@echo "Press Ctrl+C to abort, or Enter to continue..."
	@read _
	@$(call run_psql,"SELECT public.delete_tenant_data((SELECT id FROM public.tenants WHERE slug = '$(TENANT_SLUG)'));")
	@echo "Tenant data deleted."

## Development Workflow

dev: dev-local ## Alias for dev-local (default: local Docker DB)

dev-local: infra-up ## Start local Docker infrastructure and print service commands
	@echo ""
	@echo "Local infrastructure started (Docker PostgreSQL + RabbitMQ)."
	@echo "Database: localhost:5432/medflow"
	@echo ""
	@echo "Run services with:"
	@echo "  make run-api-gateway"
	@echo "  make run-auth-service"
	@echo "  make run-user-service"
	@echo "  make run-staff-service"
	@echo "  make run-inventory-service"

dev-supabase: ## Start development against Supabase DB (Usage: make dev-supabase)
	@if [ ! -f deployments/.env.supabase ]; then \
		echo "Error: deployments/.env.supabase not found."; \
		echo "Copy the template: cp deployments/.env.supabase.example deployments/.env.supabase"; \
		echo "Then fill in your Supabase credentials."; \
		exit 1; \
	fi
	@echo "Loading Supabase configuration..."
	@set -a && . deployments/.env.supabase && set +a && \
		echo "" && \
		echo "Checking Supabase connectivity..." && \
		pg_isready -h $$MEDFLOW_DATABASE_HOST -p $${MEDFLOW_DATABASE_PORT:-5432} -t 5 && \
		echo "" && \
		echo "Connected to Supabase: $$MEDFLOW_DATABASE_HOST:$${MEDFLOW_DATABASE_PORT:-5432}/$${DB_NAME:-postgres}" && \
		echo "" && \
		echo "Run services with (source env first):" && \
		echo "  source deployments/.env.supabase && make run-api-gateway" && \
		echo "  source deployments/.env.supabase && make run-auth-service" && \
		echo "  source deployments/.env.supabase && make run-user-service" && \
		echo "  source deployments/.env.supabase && make run-staff-service" && \
		echo "  source deployments/.env.supabase && make run-inventory-service" && \
		echo "" && \
		echo "Run migrations:" && \
		echo "  source deployments/.env.supabase && make migrate-up"

## Supabase Setup

supabase-setup: ## Run one-time Supabase setup (creates medflow_app role + schemas)
	@if [ ! -f deployments/.env.supabase ]; then \
		echo "Error: deployments/.env.supabase not found."; \
		echo "Copy the template: cp deployments/.env.supabase.example deployments/.env.supabase"; \
		exit 1; \
	fi
	@echo "Running Supabase setup script..."
	@set -a && . deployments/.env.supabase && set +a && \
		psql "$$MIGRATE_DATABASE_URL" -f scripts/supabase-setup.sql
	@echo ""
	@echo "Setup complete. Next steps:"
	@echo "  1. Change medflow_app password on Supabase"
	@echo "  2. Update deployments/.env.supabase with the new password"
	@echo "  3. Run migrations: source deployments/.env.supabase && make migrate-up"

## Database Status

db-status: ## Show which database is currently targeted
	@echo "Database target:"
	@echo "  Host:     $(DB_HOST)"
	@echo "  Port:     $(DB_PORT)"
	@echo "  Database: $(DB_NAME)"
	@echo "  SSL:      $(DB_SSL_MODE)"
	@echo "  Migrate:  $(DB_MIGRATE_USER)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)"
	@echo "  App:      $(DB_APP_USER)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)"
	@echo "  Local DB: $(IS_LOCAL_DB)"

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

## ============================================================
## Google Cloud Run Deployment
## ============================================================

# GCP configuration (override via environment or command line)
GCP_PROJECT ?= $(shell gcloud config get-value project 2>/dev/null)
GCP_REGION ?= europe-west1
GCP_REPOSITORY ?= medflow
REGISTRY = $(GCP_REGION)-docker.pkg.dev/$(GCP_PROJECT)/$(GCP_REPOSITORY)

# Map service names to Dockerfile suffixes
# e.g., auth-service → Dockerfile.auth, api-gateway → Dockerfile.gateway
define get_dockerfile
$(if $(filter api-gateway,$(1)),deployments/docker/Dockerfile.gateway,deployments/docker/Dockerfile.$(subst -service,,$(1)))
endef

cloud-setup: ## One-time: create Artifact Registry repo and configure Docker auth
	@echo "Creating Artifact Registry repository '$(GCP_REPOSITORY)' in $(GCP_REGION)..."
	@gcloud artifacts repositories create $(GCP_REPOSITORY) \
		--repository-format=docker \
		--location=$(GCP_REGION) \
		--description="MedFlow backend images" \
		2>/dev/null || echo "Repository already exists."
	@echo "Configuring Docker authentication..."
	@gcloud auth configure-docker $(GCP_REGION)-docker.pkg.dev --quiet
	@echo "Done. You can now run: make cloud-build-all"

cloud-build-%: ## Build and push a service image (e.g., make cloud-build-auth-service)
	@echo "Building $* → $(REGISTRY)/$*:latest"
	@docker build \
		-t $(REGISTRY)/$*:latest \
		-f $(call get_dockerfile,$*) .
	@echo "Pushing $(REGISTRY)/$*:latest"
	@docker push $(REGISTRY)/$*:latest

cloud-build-all: ## Build and push all service images
	@for service in $(SERVICES); do \
		echo "=== Building $$service ==="; \
		$(MAKE) cloud-build-$$service || exit 1; \
	done

deploy-%: ## Deploy a service to Cloud Run (e.g., make deploy-auth-service)
	@echo "Deploying $* to Cloud Run in $(GCP_REGION)..."
	@gcloud run deploy medflow-$* \
		--image=$(REGISTRY)/$*:latest \
		--region=$(GCP_REGION) \
		--platform=managed \
		--memory=256Mi \
		--cpu=1 \
		--min-instances=0 \
		--max-instances=3 \
		--set-env-vars=MEDFLOW_SERVER_ENVIRONMENT=staging \
		$(if $(filter api-gateway,$*),--allow-unauthenticated,--no-allow-unauthenticated)
	@echo "$* deployed."

deploy-all: ## Deploy all services to Cloud Run
	@for service in $(SERVICES); do \
		echo "=== Deploying $$service ==="; \
		$(MAKE) deploy-$$service || exit 1; \
	done

cloud-urls: ## Print deployed Cloud Run service URLs
	@echo "Cloud Run service URLs:"
	@for service in $(SERVICES); do \
		url=$$(gcloud run services describe medflow-$$service \
			--region=$(GCP_REGION) --format="value(status.url)" 2>/dev/null); \
		if [ -n "$$url" ]; then \
			printf "  \033[36m%-25s\033[0m %s\n" "$$service" "$$url"; \
		else \
			printf "  \033[36m%-25s\033[0m (not deployed)\n" "$$service"; \
		fi; \
	done

cloud-submit: ## Submit the full Cloud Build pipeline (builds + deploys all services)
	@echo "Submitting Cloud Build pipeline..."
	@gcloud builds submit \
		--config=cloudbuild.yaml \
		--substitutions=_REGION=$(GCP_REGION),_REPOSITORY=$(GCP_REPOSITORY) \
		.

## Vision Model Management

model-upload: ## One-time: upload vision model weights to GCS for FUSE mount
	@bash scripts/upload-model.sh
