# Admit CLI Makefile

.PHONY: all build test clean

# Default target
all: build

# Build the admit binary
build:
	go build -o admit ./cmd/admit

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-verbose:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f admit

# Build and test
check: test build
