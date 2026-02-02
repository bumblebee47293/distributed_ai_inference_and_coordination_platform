.PHONY: help build test test-coverage test-integration clean docker-build docker-up docker-down k8s-deploy k8s-delete lint

# Default target
help:
	@echo "Available targets:"
	@echo "  build              - Build all services"
	@echo "  test               - Run unit tests"
	@echo "  test-coverage      - Run tests with coverage"
	@echo "  test-integration   - Run integration tests"
	@echo "  lint               - Run linters"
	@echo "  clean              - Clean build artifacts"
	@echo "  docker-build       - Build Docker images"
	@echo "  docker-up          - Start Docker Compose environment"
	@echo "  docker-down        - Stop Docker Compose environment"
	@echo "  k8s-deploy         - Deploy to Kubernetes"
	@echo "  k8s-delete         - Delete from Kubernetes"

# Build all services
build:
	@echo "Building all services..."
	cd services/api-gateway && go build -o ../../bin/api-gateway ./cmd/main.go
	cd services/model-router && go build -o ../../bin/model-router ./cmd/main.go
	cd services/inference-orchestrator && go build -o ../../bin/inference-orchestrator ./cmd/main.go
	cd services/batch-worker && go build -o ../../bin/batch-worker ./cmd/main.go
	cd services/metadata-service && go build -o ../../bin/metadata-service ./cmd/main.go
	@echo "Build complete!"

# Run unit tests
test:
	@echo "Running unit tests..."
	go test ./services/... -v -short

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test ./services/... -v -short -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test ./tests/integration/... -v

# Lint code
lint:
	@echo "Running linters..."
	golangci-lint run ./services/...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	find . -name "*.test" -delete

# Build Docker images
docker-build:
	@echo "Building Docker images..."
	docker build -f docker/api-gateway.Dockerfile -t ai-platform/api-gateway:latest .
	docker build -f docker/model-router.Dockerfile -t ai-platform/model-router:latest .
	docker build -f docker/inference-orchestrator.Dockerfile -t ai-platform/inference-orchestrator:latest .
	docker build -f docker/batch-worker.Dockerfile -t ai-platform/batch-worker:latest .
	docker build -f docker/metadata-service.Dockerfile -t ai-platform/metadata-service:latest .

# Start Docker Compose
docker-up:
	@echo "Starting Docker Compose..."
	docker-compose up -d
	@echo "Services started! API Gateway: http://localhost:8080"

# Stop Docker Compose
docker-down:
	@echo "Stopping Docker Compose..."
	docker-compose down

# Deploy to Kubernetes
k8s-deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -k k8s/overlays/dev

# Delete from Kubernetes
k8s-delete:
	@echo "Deleting from Kubernetes..."
	kubectl delete -k k8s/overlays/dev
