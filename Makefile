.PHONY: help build run dev test clean migrate docker-up docker-down

# Detect Docker Compose command (V2 uses 'docker compose', V1 uses 'docker-compose')
DOCKER_COMPOSE := $(shell if command -v docker compose >/dev/null 2>&1; then echo "docker compose"; else echo "docker-compose"; fi)

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	go build -o bin/api cmd/api/main.go

run: ## Run the application
	go run cmd/api/main.go

dev: ## Run with live reload (requires air: go install github.com/cosmtrek/air@latest)
	air

test: ## Run tests
	go test -v ./...

clean: ## Clean build artifacts
	rm -rf bin/
	rm -rf /tmp/faceless/

migrate: ## Run database migrations
	@echo "Applying migrations..."
	psql $(DATABASE_URL) -f migrations/001_initial_schema.sql
	psql $(DATABASE_URL) -f migrations/002_add_clip_durations.sql

docker-up: ## Start all services with Docker Compose
	$(DOCKER_COMPOSE) up -d

docker-down: ## Stop all services
	$(DOCKER_COMPOSE) down

docker-logs: ## View logs from all services
	$(DOCKER_COMPOSE) logs -f

docker-rebuild: ## Rebuild and restart services
	$(DOCKER_COMPOSE) down
	$(DOCKER_COMPOSE) build --no-cache
	$(DOCKER_COMPOSE) up -d

deps: ## Download dependencies
	go mod download
	go mod tidy

fmt: ## Format code
	go fmt ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

.DEFAULT_GOAL := help
