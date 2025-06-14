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