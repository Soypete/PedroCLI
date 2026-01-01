.PHONY: build build-mac build-linux build-all test test-coverage test-coverage-report install clean run-cli run-http serve build-http build-calendar fmt lint tidy migrate-up migrate-down migrate-status migrate-reset migrate-redo db-reset db-fresh

# Default build for current platform (CLI, HTTP server, and calendar MCP server)
build:
	go build -o pedrocli ./cmd/pedrocli
	go build -o pedrocli-http-server ./cmd/http-server
	go build -o pedrocli-calendar-mcp ./cmd/calendar-mcp-server

# Build CLI only
build-cli:
	go build -o pedrocli ./cmd/pedrocli

# Build for macOS
build-mac:
	GOOS=darwin GOARCH=arm64 go build -o pedrocli-mac-arm64 ./cmd/pedrocli
	GOOS=darwin GOARCH=amd64 go build -o pedrocli-mac-amd64 ./cmd/pedrocli

# Build for Linux (Ubuntu on Spark)
build-linux:
	GOOS=linux GOARCH=amd64 go build -o pedrocli-linux-amd64 ./cmd/pedrocli

# Build HTTP server
build-http:
	go build -o pedrocli-http-server ./cmd/http-server

# Build Calendar MCP server
build-calendar:
	go build -o pedrocli-calendar-mcp ./cmd/calendar-mcp-server

# Build for both platforms
build-all: build-mac build-linux build-calendar

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
	go build -o pedrocli ./cmd/pedrocli
	sudo mv pedrocli /usr/local/bin/

# Clean build artifacts
clean:
	rm -f pedrocli pedrocli-* coverage.out coverage.html

# Run CLI (with secrets from 1Password)
run-cli:
	op run --env-file=.env -- go run cmd/pedrocli/main.go

# Run HTTP server (with secrets from 1Password)
run-http:
	op run --env-file=.env -- go run cmd/http-server/main.go

# Run HTTP server from built binary (with secrets from 1Password)
serve:
	op run --env-file=.env -- ./pedrocli-http-server

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
