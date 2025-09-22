# Telemetry System Makefile
# Module: github.com/example/telemetry

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_DIR=bin
DOCKER=docker

# Service directories
API_DIR=services/api
COLLECTOR_DIR=services/collector
MSG_QUEUE_DIR=services/msg_queue
STREAMER_DIR=services/streamer
DELETE_DATA_DIR=cmd/delete_data

# Binary names
API_BINARY=$(BINARY_DIR)/api
COLLECTOR_BINARY=$(BINARY_DIR)/collector
MSG_QUEUE_BINARY=$(BINARY_DIR)/msg_queue
STREAMER_BINARY=$(BINARY_DIR)/streamer
DELETE_DATA_BINARY=$(BINARY_DIR)/delete_data

# Windows binary names (add .exe extension)
ifeq ($(OS),Windows_NT)
    API_BINARY := $(API_BINARY).exe
    COLLECTOR_BINARY := $(COLLECTOR_BINARY).exe
    MSG_QUEUE_BINARY := $(MSG_QUEUE_BINARY).exe
    STREAMER_BINARY := $(STREAMER_BINARY).exe
    DELETE_DATA_BINARY := $(DELETE_DATA_BINARY).exe
endif

# Default target
.PHONY: all
all: clean deps build

# Create binary directory
$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

# Download dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) verify

# Build all services
.PHONY: build
build: $(BINARY_DIR) build-api build-collector build-msg-queue build-streamer build-delete-data

# Build individual services
.PHONY: build-api
build-api: $(BINARY_DIR)
	@echo "Building API service..."
	cd $(API_DIR) && $(GOBUILD) -o ../../$(API_BINARY) .

.PHONY: build-collector
build-collector: $(BINARY_DIR)
	@echo "Building Collector service..."
	cd $(COLLECTOR_DIR) && $(GOBUILD) -o ../../$(COLLECTOR_BINARY) .

.PHONY: build-msg-queue
build-msg-queue: $(BINARY_DIR)
	@echo "Building Message Queue service..."
	cd $(MSG_QUEUE_DIR) && $(GOBUILD) -o ../../$(MSG_QUEUE_BINARY) .

.PHONY: build-streamer
build-streamer: $(BINARY_DIR)
	@echo "Building Streamer service..."
	cd $(STREAMER_DIR) && $(GOBUILD) -o ../../$(STREAMER_BINARY) .

.PHONY: build-delete-data
build-delete-data: $(BINARY_DIR)
	@echo "Building Delete Data utility..."
	cd $(DELETE_DATA_DIR) && $(GOBUILD) -o ../../$(DELETE_DATA_BINARY) .

# Build with race detection
.PHONY: build-race
build-race: CGO_ENABLED=1
build-race: $(BINARY_DIR)
	@echo "Building all services with race detection..."
	cd $(API_DIR) && CGO_ENABLED=1 $(GOBUILD) -race -o ../../$(API_BINARY) .
	cd $(COLLECTOR_DIR) && CGO_ENABLED=1 $(GOBUILD) -race -o ../../$(COLLECTOR_BINARY) .
	cd $(MSG_QUEUE_DIR) && CGO_ENABLED=1 $(GOBUILD) -race -o ../../$(MSG_QUEUE_BINARY) .
	cd $(STREAMER_DIR) && CGO_ENABLED=1 $(GOBUILD) -race -o ../../$(STREAMER_BINARY) .

# Build for Linux (useful for Docker)
.PHONY: build-linux
build-linux: $(BINARY_DIR)
	@echo "Building all services for Linux..."
	cd $(API_DIR) && GOOS=linux GOARCH=amd64 $(GOBUILD) -o ../../$(BINARY_DIR)/api-linux .
	cd $(COLLECTOR_DIR) && GOOS=linux GOARCH=amd64 $(GOBUILD) -o ../../$(BINARY_DIR)/collector-linux .
	cd $(MSG_QUEUE_DIR) && GOOS=linux GOARCH=amd64 $(GOBUILD) -o ../../$(BINARY_DIR)/msg_queue-linux .
	cd $(STREAMER_DIR) && GOOS=linux GOARCH=amd64 $(GOBUILD) -o ../../$(BINARY_DIR)/streamer-linux .

# Run tests
.PHONY: test
test:
	@echo "Running all tests..."
	$(GOTEST) -v ./services/...

.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./services/...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: test-race
test-race:
	@echo "Running tests with race detection..."
	CGO_ENABLED=1 $(GOTEST) -v -race ./services/...

# Clean up
.PHONY: clean
clean:
	@echo "Cleaning up..."
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

# Generate Swagger docs
.PHONY: swagger
swagger:
	@echo "Generating Swagger documentation..."
	swag init -g $(API_DIR)/main.go -o $(API_DIR)/docs

# Docker builds
.PHONY: docker-build
docker-build:
	@echo "Building Docker images..."
	$(DOCKER) build -t telemetry-api -f $(API_DIR)/Dockerfile .
	$(DOCKER) build -t telemetry-collector -f $(COLLECTOR_DIR)/Dockerfile .
	$(DOCKER) build -t telemetry-msg-queue -f $(MSG_QUEUE_DIR)/Dockerfile .
	$(DOCKER) build -t telemetry-streamer -f $(STREAMER_DIR)/Dockerfile .

# Run services locally
.PHONY: run-api
run-api: build-api
	@echo "Running API service..."
	./$(API_BINARY)

.PHONY: run-collector
run-collector: build-collector
	@echo "Running Collector service..."
	./$(COLLECTOR_BINARY)

.PHONY: run-msg-queue
run-msg-queue: build-msg-queue
	@echo "Running Message Queue service..."
	./$(MSG_QUEUE_BINARY)

.PHONY: run-streamer
run-streamer: build-streamer
	@echo "Running Streamer service..."
	./$(STREAMER_BINARY)

# Development helpers
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

.PHONY: vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run

.PHONY: mod-tidy
mod-tidy:
	@echo "Tidying go.mod..."
	$(GOMOD) tidy

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all           - Clean, download deps, and build all services"
	@echo "  build         - Build all services"
	@echo "  build-api     - Build API service only"
	@echo "  build-collector - Build Collector service only"
	@echo "  build-msg-queue - Build Message Queue service only"
	@echo "  build-streamer - Build Streamer service only"
	@echo "  build-delete-data - Build Delete Data utility only"
	@echo "  build-race    - Build with race detection"
	@echo "  build-linux   - Build for Linux (Docker)"
	@echo "  test          - Run all tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-race     - Run tests with race detection"
	@echo "  clean         - Clean up binaries and artifacts"
	@echo "  deps          - Download and verify dependencies"
	@echo "  swagger       - Generate Swagger documentation"
	@echo "  docker-build  - Build Docker images"
	@echo "  run-*         - Run individual services"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run golangci-lint"
	@echo "  mod-tidy      - Tidy go.mod"
	@echo "  help          - Show this help"
