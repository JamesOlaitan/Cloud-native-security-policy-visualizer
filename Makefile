.PHONY: build test lint sec ui dev demo demo-diff clean

# Build all binaries
build:
	@echo "Building binaries..."
	@mkdir -p bin
	go build -o bin/accessgraph-api ./cmd/accessgraph-api
	go build -o bin/accessgraph-ingest ./cmd/accessgraph-ingest
	go build -o bin/accessgraph-cli ./cmd/accessgraph-cli
	@echo "Build complete!"

# Run tests with coverage
test:
	@echo "Running tests..."
	go test -v -cover ./... -coverprofile=coverage.out
	@echo "Tests complete!"
	@go tool cover -func=coverage.out | grep total:

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run
	@echo "Lint complete!"

# Run security scanner
sec:
	@echo "Running security scanner..."
	gosec ./...
	@echo "Security scan complete!"

# Build UI
ui:
	@echo "Building UI..."
	cd ui && npm install && npm run build
	@echo "UI build complete!"

# Generate GraphQL code
generate:
	@echo "Generating GraphQL code..."
	go run github.com/99designs/gqlgen generate
	@echo "GraphQL generation complete!"

# Start development environment
dev:
	@echo "Starting development environment..."
	docker compose up --build

# Run demo ingestion
demo:
	@echo "Running demo ingestion..."
	@mkdir -p data
	@echo "Ingesting demo1 snapshot..."
	go run ./cmd/accessgraph-ingest --aws sample/aws --k8s sample/k8s --snapshot demo1
	@echo "Ingesting demo2 snapshot (with Terraform)..."
	go run ./cmd/accessgraph-ingest --aws sample/aws --k8s sample/k8s --tf sample/terraform/plan.json --snapshot demo2
	@echo "Listing snapshots..."
	go run ./cmd/accessgraph-cli snapshots ls
	@echo "Demo ingestion complete!"

# Run demo diff
demo-diff:
	@echo "Running snapshot diff..."
	go run ./cmd/accessgraph-cli snapshots diff --a demo1 --b demo2
	@echo "Diff complete!"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out
	rm -f *.db
	rm -rf ui/dist
	rm -rf ui/node_modules
	@echo "Clean complete!"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "Dependencies installed!"

# Fix dependency issues
fix:
	@echo "Fixing dependencies..."
	go clean -modcache
	go mod download
	go mod tidy
	go mod verify
	cd ui && npm install && cd ..
	@echo "Dependencies fixed!"

