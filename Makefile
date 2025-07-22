.PHONY: help build run-publisher run-consumer test clean docker-up docker-down

APP_NAME=gohopper
BUILD_DIR=build


help: 
	@echo "Comandos disponíveis:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the project
	@echo "Compiling $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/publisher ./cmd/publisher
	go build -o $(BUILD_DIR)/consumer ./cmd/consumer
	@echo "Compilation completed!"

run-publisher: ## Run the publisher
	@echo "Running publisher..."
	go run ./cmd/publisher

run-consumer: ## Run the consumer
	@echo "Running consumer..."
	go run ./cmd/consumer

publish: ## Simulate event publishing (CLI)
	@echo "Publishing test events..."
	@go run ./cmd/publisher -cli

test: ## Run the tests
	@echo "Running tests..."
	go test -v ./...

test-coverage: ## Run the tests with coverage
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean: ## Clean the build files
	@echo "Cleaning build files..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "Cleanup completed!"

# Docker commands
docker-up: ## Start RabbitMQ via Docker
	@echo "Starting RabbitMQ..."
	docker-compose up -d
	@echo "RabMQ started at http://localhost:15672"

docker-down: ## Stop RabbitMQ via Docker
	@echo "Stopping RabbitMQ..."
	docker-compose down
	@echo "RabbitMQ stopped"

docker-logs: ## Show RabbitMQ logs
	@echo "RabbitMQ logs:"
	docker-compose logs -f rabbitmq

# Development commands
deps: ## Install dependencies
	@echo "Installing dependencies..."
	go mod download
	@echo "Dependencies installed!"

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy
	@echo "Dependencies updated!"

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run
	@echo "Linter completed!"

format: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	@echo "Formatting completed!"

# Setup commands
setup: ## Configure the development environment
	@echo "Configuring environment..."
	@if [ ! -f .env ]; then cp .env.example .env; echo "📝 .env file created"; fi
	@echo "Environment configured!"

dev: setup docker-up ## Configure and start the development environment
	@echo "Development environment ready!"
	@echo "RabbitMQ Management: http://localhost:15672"
	@echo "To run consumer: make run-consumer (in one terminal)"
	@echo "To run publisher: make run-publisher (in another terminal)"
	@echo "To publish test events: make publish" 