.PHONY: build build-mac build-linux build-all test test-postgres-up test-postgres-down test-postgres-clean test-with-postgres test-coverage test-coverage-report install clean run-cli run-http serve build-http build-calendar fmt lint tidy migrate-up migrate-down migrate-status migrate-reset migrate-redo db-reset db-fresh postgres-up postgres-down postgres-logs postgres-shell llama-server llama-health stop-llama whisper-server whisper-health stop-whisper

# llama-server configuration
LLAMA_PORT ?= 8082
LLAMA_MODEL ?= $(shell find ~/.cache/huggingface/hub/models--bartowski--Qwen2.5-Coder-32B-Instruct-GGUF -name "*.gguf" -type f | head -1)
LLAMA_CTX_SIZE ?= 16384
LLAMA_N_GPU_LAYERS ?= -1
LLAMA_THREADS ?= 8
LLAMA_GRAMMAR ?=
LLAMA_GRAMMAR_FILE ?=
LLAMA_LOGIT_BIAS ?=

# llama-server targets
llama-server: ## Start llama-server for tool calling
	@echo "Starting llama-server on port $(LLAMA_PORT)..."
	@if [ -z "$(LLAMA_MODEL)" ]; then \
		echo "Error: LLAMA_MODEL not found"; \
		exit 1; \
	fi
	@llama-server \
		--model $(LLAMA_MODEL) \
		--port $(LLAMA_PORT) \
		--ctx-size $(LLAMA_CTX_SIZE) \
		--n-gpu-layers $(LLAMA_N_GPU_LAYERS) \
		--threads $(LLAMA_THREADS) \
		--jinja \
		--log-disable \
		--no-webui \
		--metrics

llama-health: ## Check llama-server health
	@curl -s http://localhost:$(LLAMA_PORT)/health | jq . || echo "Server not running"

stop-llama: ## Stop llama-server
	@pkill -f llama-server || echo "No llama-server running"

# whisper.cpp configuration
WHISPER_PORT ?= 8081
WHISPER_BIN ?= ~/Code/ml/whisper.cpp/build/bin/whisper-server
WHISPER_MODEL ?= ~/Code/ml/whisper.cpp/models/ggml-base.en.bin

# whisper.cpp targets
whisper-server: ## Start whisper.cpp server for voice transcription
	@echo "Starting whisper-server on port $(WHISPER_PORT)..."
	@$(WHISPER_BIN) \
		--model $(WHISPER_MODEL) \
		--port $(WHISPER_PORT) \
		--convert

whisper-health: ## Check whisper-server health
	@curl -s http://localhost:$(WHISPER_PORT)/health || echo "Server not running"

stop-whisper: ## Stop whisper-server
	@pkill -f whisper-server || echo "No whisper-server running"

# Default build for current platform (CLI, HTTP server, and REPL)
build:
	go build -o pedrocli ./cmd/pedrocli
	go build -o pedrocli-http-server ./cmd/http-server
	go build -o pedrocode ./cmd/pedrocode

# Build CLI only
build-cli:
	go build -o pedrocli ./cmd/pedrocli

# Build REPL (pedrocode)
build-pedrocode:
	go build -o pedrocode ./cmd/pedrocode

# Build for macOS
build-mac:
	GOOS=darwin GOARCH=arm64 go build -o pedrocli-mac-arm64 ./cmd/pedrocli
	GOOS=darwin GOARCH=amd64 go build -o pedrocli-mac-amd64 ./cmd/pedrocli
	GOOS=darwin GOARCH=arm64 go build -o pedrocode-mac-arm64 ./cmd/pedrocode
	GOOS=darwin GOARCH=amd64 go build -o pedrocode-mac-amd64 ./cmd/pedrocode

# Build for Linux (Ubuntu on Spark)
build-linux:
	GOOS=linux GOARCH=amd64 go build -o pedrocli-linux-amd64 ./cmd/pedrocli
	GOOS=linux GOARCH=amd64 go build -o pedrocode-linux-amd64 ./cmd/pedrocode

# Build HTTP server
build-http:
	go build -o pedrocli-http-server ./cmd/http-server

# Build for both platforms
build-all: build-mac build-linux

# PostgreSQL Test Infrastructure (Podman)
test-postgres-up:
	@echo "Starting PostgreSQL test container with Podman..."
	@podman run -d \
		--name pedrocli-postgres-test \
		-e POSTGRES_USER=pedrocli_test \
		-e POSTGRES_PASSWORD=pedrocli_test \
		-e POSTGRES_DB=pedrocli_test \
		-p 5433:5432 \
		postgres:15-alpine
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@until podman exec pedrocli-postgres-test pg_isready -U pedrocli_test > /dev/null 2>&1; do \
		echo "Waiting for PostgreSQL..."; \
		sleep 1; \
	done
	@echo "PostgreSQL is ready!"

test-postgres-down:
	@echo "Stopping and removing PostgreSQL test container..."
	@podman stop pedrocli-postgres-test 2>/dev/null || true
	@podman rm pedrocli-postgres-test 2>/dev/null || true

test-postgres-clean: test-postgres-down
	@echo "Cleaning up PostgreSQL test volumes..."
	@podman volume prune -f

test-with-postgres: test-postgres-up
	@echo "Running tests with PostgreSQL..."
	@export TEST_DATABASE_URL="postgres://pedrocli_test:pedrocli_test@localhost:5433/pedrocli_test?sslmode=disable" && \
	go test -v -count=1 ./...
	@$(MAKE) test-postgres-down

# Run tests
test: test-with-postgres

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
	rm -f pedrocli pedrocli-* pedrocode pedrocode-* coverage.out coverage.html

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

# PostgreSQL Docker Management
postgres-up:
	@echo "Starting PostgreSQL..."
	@mkdir -p postgres-data
	@docker run -d \
		--name pedrocli-postgres \
		-p 5432:5432 \
		-e POSTGRES_USER=pedrocli \
		-e POSTGRES_PASSWORD=pedrocli \
		-e POSTGRES_DB=pedrocli \
		-v $(PWD)/postgres-data:/var/lib/postgresql/data \
		postgres:15-alpine || echo "Container may already exist"
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@docker exec pedrocli-postgres pg_isready -U pedrocli || sleep 2
	@echo "PostgreSQL is ready at postgres://pedrocli:pedrocli@localhost:5432/pedrocli"

postgres-down:
	@echo "Stopping PostgreSQL..."
	@docker stop pedrocli-postgres || true
	@docker rm pedrocli-postgres || true

postgres-logs:
	docker logs -f pedrocli-postgres

postgres-shell:
	docker exec -it pedrocli-postgres psql -U pedrocli -d pedrocli
