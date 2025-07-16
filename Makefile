# FinCache Makefile
# Advanced Redis-compatible in-memory cache

.PHONY: help build test clean docker-build docker-run docker-stop docker-clean benchmark deploy

# Variables
BINARY_NAME=fincache
DOCKER_IMAGE=fincache
DOCKER_TAG=latest
GO_VERSION=1.21

# Default target
help: ## Show this help message
	@echo "🏦 FinCache - Advanced Redis-compatible in-memory cache"
	@echo "=================================================="
	@echo ""
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development commands
build: ## Build the FinCache binary
	@echo "🔨 Building FinCache..."
	go mod tidy
	go build -o bin/$(BINARY_NAME) ./cmd/fincache
	@echo "✅ Build complete: bin/$(BINARY_NAME)"

run: build ## Build and run FinCache locally
	@echo "🚀 Starting FinCache..."
	./bin/$(BINARY_NAME)

test: ## Run tests
	@echo "🧪 Running tests..."
	go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "🧪 Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report: coverage.html"

lint: ## Run linter
	@echo "🔍 Running linter..."
	golangci-lint run

fmt: ## Format code
	@echo "🎨 Formatting code..."
	go fmt ./...

# Docker commands
docker-build: ## Build Docker image
	@echo "🐳 Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "✅ Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

docker-run: ## Run FinCache with Docker Compose
	@echo "🚀 Starting FinCache with Docker Compose..."
	docker-compose up -d
	@echo "✅ Services started!"
	@echo "📊 FinCache: http://localhost:8080"
	@echo "🔴 Redis Protocol: localhost:6379"
	@echo "📈 Grafana: http://localhost:3000 (admin/admin)"
	@echo "📊 Prometheus: http://localhost:9090"

docker-stop: ## Stop Docker services
	@echo "🛑 Stopping Docker services..."
	docker-compose down

docker-clean: ## Clean Docker resources
	@echo "🧹 Cleaning Docker resources..."
	docker-compose down -v --rmi all
	docker system prune -f

# Testing and benchmarking
test-integration: ## Run integration tests
	@echo "🧪 Running integration tests..."
	@chmod +x scripts/test.sh
	./scripts/test.sh

benchmark: ## Run performance benchmarks
	@echo "⚡ Running performance benchmarks..."
	@chmod +x scripts/benchmark.sh
	./scripts/benchmark.sh

# Deployment commands
deploy-local: docker-build docker-run ## Deploy locally with Docker
	@echo "✅ FinCache deployed locally!"

deploy-production: ## Deploy to production (example)
	@echo "🚀 Deploying to production..."
	@echo "⚠️  Configure production deployment here"

# Utility commands
clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean

logs: ## Show FinCache logs
	@echo "📋 Showing FinCache logs..."
	docker-compose logs -f fincache

status: ## Show service status
	@echo "📊 Service Status:"
	@echo "FinCache Health:"
	@curl -s http://localhost:8080/health || echo "❌ FinCache not running"
	@echo ""
	@echo "FinCache Stats:"
	@curl -s http://localhost:8080/api/v1/stats || echo "❌ Stats not available"
	@echo ""
	@echo "Redis Protocol:"
	@redis-cli -h localhost -p 6379 PING || echo "❌ Redis protocol not responding"

# Development setup
setup: ## Setup development environment
	@echo "🔧 Setting up development environment..."
	@echo "Installing dependencies..."
	go mod download
	@echo "Creating directories..."
	mkdir -p bin data
	@echo "Setting up monitoring..."
	mkdir -p monitoring/grafana/dashboards monitoring/grafana/datasources
	@echo "✅ Development environment ready!"

# Quick start
quickstart: setup build run ## Quick start: setup, build, and run

# Documentation
docs: ## Generate documentation
	@echo "📚 Generating documentation..."
	@echo "API Documentation: http://localhost:8080/sandbox"
	@echo "Health Check: http://localhost:8080/health"
	@echo "Metrics: http://localhost:8080/metrics"

# Monitoring
monitor: ## Open monitoring dashboards
	@echo "📊 Opening monitoring dashboards..."
	@echo "Grafana: http://localhost:3000 (admin/admin)"
	@echo "Prometheus: http://localhost:9090"
	@echo "FinCache Stats: http://localhost:8080/api/v1/stats"

# Performance testing
load-test: ## Run load testing
	@echo "🔥 Running load test..."
	redis-benchmark -h localhost -p 6379 -n 100000 -c 100 -t SET,GET

# Security
security-scan: ## Run security scan
	@echo "🔒 Running security scan..."
	@echo "⚠️  Implement security scanning here"

# Backup and restore
backup: ## Backup FinCache data
	@echo "💾 Creating backup..."
	@echo "⚠️  Implement backup logic here"

restore: ## Restore FinCache data
	@echo "📥 Restoring data..."
	@echo "⚠️  Implement restore logic here" 