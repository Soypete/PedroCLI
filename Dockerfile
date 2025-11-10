# Multi-stage Dockerfile for PedroCLI
# Build: docker build -t pedrocli .
# Run: docker run -v /path/to/project:/workspace -v ~/.pedroceli.json:/root/.pedroceli.json pedrocli

# Stage 1: Build
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o pedrocli ./cmd/pedrocli
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o pedrocli-server ./cmd/mcp-server

# Stage 2: Runtime
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    git \
    curl \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 pedrocli && \
    adduser -D -u 1000 -G pedrocli pedrocli

WORKDIR /workspace

# Copy binaries from builder
COPY --from=builder /build/pedrocli /usr/local/bin/
COPY --from=builder /build/pedrocli-server /usr/local/bin/

# Copy example configs
COPY .pedroceli.example.ollama.json /usr/share/pedrocli/
COPY .pedroceli.example.llamacpp.json /usr/share/pedrocli/

# Switch to non-root user
USER pedrocli

# Default command
ENTRYPOINT ["/usr/local/bin/pedrocli"]
CMD ["help"]

# Labels
LABEL org.opencontainers.image.title="PedroCLI"
LABEL org.opencontainers.image.description="Self-hosted autonomous coding agent"
LABEL org.opencontainers.image.source="https://github.com/Soypete/PedroCLI"
LABEL org.opencontainers.image.licenses="MIT"
