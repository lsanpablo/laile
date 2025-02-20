# Simple Makefile for a Go project

# Build the application
all: build

# Build for development with Air
build-dev:
	@echo "Building for development..."
	@go build -o tmp/main cmd/api/main.go

# Build for production
build:
	@echo "Building for production..."
	@go build -o main cmd/api/main.go

# Run the application
run:
	@go run cmd/api/main.go

# Run with Air for development
dev:
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Installing air..." && \
		go install github.com/cosmtrek/air@latest && \
		air; \
	fi

# Create DB container
docker-run:
	@if docker compose up 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose up; \
	fi

# Shutdown DB container
docker-down:
	@if docker compose down 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose down; \
	fi

# Test the application
test:
	@echo "Testing..."
	@go test ./tests -v

# Clean the binary
clean:
	@echo "Cleaning..."
	@rm -f main
	@rm -rf tmp

apply-migration:
	@echo "Applying migration..."
	goose -dir internal/db_models/migrations postgres "postgresql://luis:password1234@localhost:5432/laile?sslmode=disable" up

.PHONY: all build build-dev run dev test clean docker-run docker-down apply-migration