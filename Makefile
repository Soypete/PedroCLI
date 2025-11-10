.PHONY: build build-mac build-linux build-all test install clean run-server run-cli

# Default build for current platform (CLI and server)
build:
	go build -o pedrocli cmd/pedrocli/main.go
	go build -o pedrocli-server cmd/mcp-server/main.go

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

# Build for both platforms
build-all: build-mac build-linux build-server

# Run tests
test:
	go test ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Install locally
install:
	go build -o pedrocli cmd/pedrocli/main.go
	sudo mv pedrocli /usr/local/bin/

# Clean build artifacts
clean:
	rm -f pedrocli pedrocli-* coverage.out

# Run MCP server
run-server:
	go run cmd/mcp-server/main.go

# Run CLI
run-cli:
	go run cmd/pedrocli/main.go

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Tidy dependencies
tidy:
	go mod tidy
