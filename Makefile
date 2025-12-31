.PHONY: build build-mac build-linux build-all test test-coverage test-coverage-report install clean run-server run-cli run-http build-http build-calendar fmt lint tidy migrate-up migrate-down migrate-status migrate-reset migrate-redo db-reset db-fresh

# Default build for current platform (CLI, server, HTTP server, and calendar MCP server)
build:
	go build -o pedrocli cmd/pedrocli/main.go
	go build -o pedrocli-server cmd/mcp-server/main.go
	go build -o pedrocli-http-server cmd/http-server/main.go
	go build -o pedrocli-calendar-mcp cmd/calendar-mcp-server/main.go

# Build CLI only
build-cli:
	go build -o pedrocli cmd/pedrocli/main.go

# Build for macOS
build-mac:
	GOOS=darwin GOARCH=arm64 go build -o pedrocli-mac-arm64 cmd/pedrocli/main.go
	GOOS=darwin GOARCH=amd64 go build -o pedrocli-mac-amd64 cmd/pedrocli/main.go

# Build for Linux (Ubuntu on Spark)
build-linux:
	GOOS=linux GOARCH=amd64 go build -o pedrocli-linux-amd64 cmd/pedrocli/main.go

# Build MCP server
build-server:
	go build -o pedrocli-server cmd/mcp-server/main.go

# Build HTTP server
build-http:
	go build -o pedrocli-http-server cmd/http-server/main.go

# Build Calendar MCP server
build-calendar:
	go build -o pedrocli-calendar-mcp cmd/calendar-mcp-server/main.go

# Build for both platforms
build-all: build-mac build-linux build-server build-calendar

# Run tests
test:
	go test ./...

# Run tests with coverage
test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Show coverage percentage
test-coverage-report:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out | grep total | awk '{print "Total Coverage: " $$3}'

# Install locally
install:
	go build -o pedrocli cmd/pedrocli/main.go
	sudo mv pedrocli /usr/local/bin/

# Clean build artifacts
clean:
	rm -f pedrocli pedrocli-* coverage.out coverage.html

# Run MCP server
run-server:
	go run cmd/mcp-server/main.go

# Run CLI
run-cli:
	go run cmd/pedrocli/main.go

# Run HTTP server
run-http:
	go run cmd/http-server/main.go

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Tidy dependencies
tidy:
	go mod tidy

# Database migrations
migrate-up:
	go run cmd/pedrocli/main.go migrate up

migrate-down:
	go run cmd/pedrocli/main.go migrate down

migrate-status:
	go run cmd/pedrocli/main.go migrate status

migrate-reset:
	go run cmd/pedrocli/main.go migrate reset

migrate-redo:
	go run cmd/pedrocli/main.go migrate redo

# Development database reset
db-reset: migrate-reset migrate-up
	@echo "Database reset complete"

# Fresh database (delete and recreate)
db-fresh:
	rm -f /var/pedro/repos/pedro.db
	go run cmd/pedrocli/main.go migrate up
	@echo "Fresh database created"
