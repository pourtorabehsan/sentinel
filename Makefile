.PHONY: build run test lint clean test-e2e

# Build variables
BINARY_NAME=sentinel
BUILD_DIR=bin
GO_FILES=$(shell find . -name '*.go' -not -path "./vendor/*")

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Build flags
LDFLAGS=-ldflags "-s -w"

build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) cmd/sentinel/main.go

run: build
	@./$(BUILD_DIR)/$(BINARY_NAME)

test:
	@echo "Running tests..."
	@$(GOTEST) -v ./...

test-e2e:
	@echo "Running end-to-end tests..."
	@$(GOTEST) -v ./test/e2e -count=1

lint:
	@echo "Running linter..."
	@$(GOLINT) run

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)

tidy:
	@echo "Tidying up modules..."
	@$(GOMOD) tidy

dev: build
	@./$(BUILD_DIR)/$(BINARY_NAME) -config example.yaml
