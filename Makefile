# Makefile for Anime Upscale CLI tool

.PHONY: all build install clean test run lint help

BINARY_NAME=au.exe
CMD_PATH=.
ALIAS_PATH=./cmd/au

all: build

build:
	@echo "Building main CLI..."
	go build -o $(BINARY_NAME) $(CMD_PATH)
	@echo "Building alias CLI..."
	go build -o cmd/au/$(BINARY_NAME) $(ALIAS_PATH)
	@echo "Build complete."

install:
	@echo "Installing animeupscale to GOPATH..."
	go install $(CMD_PATH)
	@echo "Installing au alias to GOPATH..."
	go install $(ALIAS_PATH)
	@echo "Installation complete."

clean:
	@echo "Cleaning binaries and temp directories..."
	@if exist $(BINARY_NAME) del $(BINARY_NAME)
	@if exist cmd\au\$(BINARY_NAME) del cmd\au\$(BINARY_NAME)
	@if exist temp rmdir /s /q temp
	@if exist bench.json del bench.json
	@echo "Clean complete."

test:
	@echo "Running tests..."
	go test ./...

run: build
	./$(BINARY_NAME) -list-engines

lint:
	@echo "Linting/formatting code..."
	go fmt ./...
	go vet ./...

help:
	@echo "Available commands:"
	@echo "  make build    - Build executables (au.exe) locally"
	@echo "  make install  - Install binaries to GOPATH/bin"
	@echo "  make clean    - Remove build artifacts and temporary frames"
	@echo "  make test     - Run Go package tests"
	@echo "  make run      - Build and run CLI to list detected engines"
	@echo "  make lint     - Format and vet Go codebase"
