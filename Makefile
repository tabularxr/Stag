.PHONY: build run test clean deps docker-build docker-up docker-down

# Build variables
BINARY_NAME=stag
DOCKER_IMAGE=tabular/stag:v2

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build the binary
build:
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/stag

# Run the application
run: build
	./$(BINARY_NAME)

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE) .

# Start services with Docker Compose
docker-up:
	docker-compose up -d

# Stop services
docker-down:
	docker-compose down

# Start only ArangoDB
docker-up-db:
	docker-compose up -d arangodb

# View logs
docker-logs:
	docker-compose logs -f

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Run with hot reload (requires air)
dev:
	air