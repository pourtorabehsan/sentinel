.PHONY: all build run test clean fmt vet lint help

# Binary name
BINARY_NAME=sentinel
BINARY_PATH=bin/$(BINARY_NAME)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build parameters
BUILD_DIR=bin
CMD_DIR=cmd/sentinel
CONFIG_FILE=config.yaml

all: test build

## build: Build the sentinel binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BINARY_PATH) ./$(CMD_DIR)
	@echo "Build complete: $(BINARY_PATH)"

## run: Run the application with example config
run: build
	@echo "Running $(BINARY_NAME)..."
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "Error: $(CONFIG_FILE) not found. Please create a config file first."; \
		exit 1; \
	fi
	./$(BINARY_PATH) --config=$(CONFIG_FILE)

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

## coverage: Run tests with coverage report
coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

## fmt: Format all Go source files
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...
	@echo "Format complete"

## vet: Run go vet on all packages
vet:
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "Vet complete"

## lint: Run golint (if available)
lint:
	@echo "Running linters..."
	@which golint > /dev/null 2>&1 && golint ./... || echo "golint not installed, skipping"
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"
	@echo "Lint complete"

## check: Run fmt, vet, and test
check: fmt vet test
	@echo "All checks passed"

## deps: Download and verify dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOCMD) mod download
	$(GOCMD) mod verify
	@echo "Dependencies ready"

## tidy: Tidy go.mod and go.sum
tidy:
	@echo "Tidying dependencies..."
	$(GOCMD) mod tidy
	@echo "Tidy complete"

## install: Install the binary to $GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install ./$(CMD_DIR)
	@echo "Install complete"

## help: Show this help message
help:
	@echo "Sentinel Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/  /'
