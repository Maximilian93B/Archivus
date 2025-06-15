# Archivus Makefile
.PHONY: help build run dev test clean deps docker-build docker-run migrate

# Default target
help: ## Show help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development commands
deps: ## Install dependencies
	go mod download
	go mod tidy

build: ## Build the application
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/archivus cmd/server/main.go

run: ## Run the application
	go run cmd/server/main.go

dev: ## Run in development mode with auto-reload (requires air)
	air -c .air.toml

test: ## Run tests
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

# Database commands
migrate-up: ## Run database migrations up
	go run cmd/migrate/main.go up

migrate-down: ## Run database migrations down
	go run cmd/migrate/main.go down

migrate-create: ## Create a new migration (usage: make migrate-create NAME=create_users_table)
	go run cmd/migrate/main.go create $(NAME)

# Docker commands
docker-build: ## Build Docker image
	docker build -t archivus:latest .

docker-run: ## Run application in Docker
	docker-compose up --build

docker-dev: ## Run development environment with Docker
	docker-compose -f docker-compose.dev.yml up --build

docker-down: ## Stop Docker containers
	docker-compose down

# Environment setup
setup-dev: ## Setup development environment
	cp env.example .env
	@echo "Please edit .env file with your configuration"

# Linting and formatting
fmt: ## Format code
	go fmt ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

# Security
security-check: ## Run security checks (requires gosec)
	gosec ./...

# Build for different platforms
build-linux: ## Build for Linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/archivus-linux cmd/server/main.go

build-windows: ## Build for Windows
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/archivus-windows.exe cmd/server/main.go

build-darwin: ## Build for macOS
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/archivus-darwin cmd/server/main.go

build-all: build-linux build-windows build-darwin ## Build for all platforms

# Database setup for different environments
db-setup-dev: ## Setup development database
	createdb archivus_dev || true
	createdb archivus_test || true

db-reset-dev: ## Reset development database
	dropdb archivus_dev || true
	dropdb archivus_test || true
	make db-setup-dev
	make migrate-up

# Supabase commands
verify-supabase: ## Verify Supabase connection and setup
	go run scripts/verify-supabase.go

setup-supabase: ## Setup environment for Supabase deployment
	cp env.supabase.example .env
	@echo "Please edit .env with your Supabase credentials from:"
	@echo "https://supabase.com/dashboard → Your Project → Settings → Database"
	@echo ""
	@echo "Then run: make verify-supabase"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Project variables
BINARY_NAME=archivus
BINARY_UNIX=$(BINARY_NAME)_unix
SERVER_MAIN=./cmd/server
MIGRATE_MAIN=./cmd/migrate

.PHONY: all build clean test coverage deps run-server run-migrate-up run-migrate-down run-migrate-reset db-test

all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(SERVER_MAIN)

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v $(SERVER_MAIN)

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run repository tests only
test-repos:
	$(GOTEST) -v ./internal/infrastructure/repositories/postgresql/...

# Run integration tests
test-integration:
	$(GOTEST) -v ./test/integration/...

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy
	$(GOGET) github.com/nedpals/supabase-go

# Development commands
run-server:
	$(GOCMD) run $(SERVER_MAIN)/main.go

run-migrate-up:
	$(GOCMD) run $(MIGRATE_MAIN)/main.go up

run-migrate-down:
	$(GOCMD) run $(MIGRATE_MAIN)/main.go down

run-migrate-reset:
	$(GOCMD) run $(MIGRATE_MAIN)/main.go reset

run-migrate-seed:
	$(GOCMD) run $(MIGRATE_MAIN)/main.go seed

run-migrate-status:
	$(GOCMD) run $(MIGRATE_MAIN)/main.go status

# Database setup for testing
db-test-setup:
	@echo "Setting up test database..."
	@echo "Make sure you have DATABASE_URL_TEST configured in your .env file"

# Docker commands
docker-build:
	docker build -t archivus:latest .

docker-run:
	docker-compose up -d

docker-stop:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Development helpers
dev-setup: deps run-migrate-up run-migrate-seed
	@echo "Development environment setup complete!"

dev-test: test-repos test-integration
	@echo "All tests passed!"

# Clean everything
clean-all: clean
	docker-compose down -v
	rm -f coverage.out coverage.html