.PHONY: build build-mac build-linux build-all test install clean run-server

# Default build for current platform
build:
	go build -o pedroceli cmd/cli/main.go

# Build for macOS
build-mac:
	GOOS=darwin GOARCH=arm64 go build -o pedroceli-mac-arm64 cmd/cli/main.go
	GOOS=darwin GOARCH=amd64 go build -o pedroceli-mac-amd64 cmd/cli/main.go

# Build for Linux (Ubuntu on Spark)
build-linux:
	GOOS=linux GOARCH=amd64 go build -o pedroceli-linux-amd64 cmd/cli/main.go

# Build MCP server
build-server:
	go build -o pedroceli-server cmd/mcp-server/main.go

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
	go build -o pedroceli cmd/cli/main.go
	sudo mv pedroceli /usr/local/bin/

# Clean build artifacts
clean:
	rm -f pedroceli pedroceli-* coverage.out

# Run MCP server
run-server:
	go run cmd/mcp-server/main.go

# Run CLI
run:
	go run cmd/cli/main.go

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Tidy dependencies
tidy:
	go mod tidy
