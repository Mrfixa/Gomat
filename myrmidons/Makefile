# Myrmidons Makefile

.PHONY: all build test clean run migrate docker fmt vet

# Variables
BINARY_NAME=myrmidons
MAIN_PATH=./cmd/server
DOCKER_IMAGE=myrmidons/market

all: build

build:
	go build -ldflags="-w -s" -o $(BINARY_NAME) $(MAIN_PATH)

build-dev:
	go build -o $(BINARY_NAME) $(MAIN_PATH)

test:
	go test -v -race ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

run: build
	./$(BINARY_NAME)

run-dev: build-dev
	./$(BINARY_NAME) --dev

fmt:
	go fmt ./...

vet:
	go vet ./...

deps:
	go mod download
	go mod tidy

help:
	@echo "Myrmidons Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build        - Build the binary"
	@echo "  make run          - Build and run"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make fmt          - Format code"
	@echo "  make vet          - Run go vet"
