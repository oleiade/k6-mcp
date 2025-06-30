# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies including those needed for CGO and ChromaDB client
RUN apk add --no-cache \
    git \
    ca-certificates \
    build-base \
    gcc \
    musl-dev

# Set working directory
WORKDIR /build

# Copy go modules files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application with CGO enabled for ChromaDB dependencies
RUN CGO_ENABLED=1 go build \
    -ldflags='-w -s' \
    -o k6-mcp \
    ./main.go

# Final stage - use alpine for runtime dependencies
FROM alpine:latest

# Install runtime dependencies for CGO binaries
RUN apk add --no-cache ca-certificates

# Install k6 binary
COPY --from=grafana/k6:latest /usr/bin/k6 /usr/bin/k6

# Copy the binary from builder stage
COPY --from=builder /build/k6-mcp /usr/local/bin/k6-mcp

# Copy the k6 documentation
COPY --from=builder /build/k6-docs /app/k6-docs

# Make binary executable
RUN chmod +x /usr/local/bin/k6-mcp

# Create non-root user and writable cache directory
RUN addgroup -g 65532 nonroot && \
    adduser -D -u 65532 -G nonroot nonroot && \
    mkdir -p /app/cache && \
    chown -R nonroot:nonroot /app/cache

# Set environment variables for cache directories
ENV HF_HOME=/app/cache/huggingface \
    TRANSFORMERS_CACHE=/app/cache/transformers \
    HF_DATASETS_CACHE=/app/cache/datasets \
    TOKENIZERS_CACHE=/app/cache/tokenizers

# Use non-root user for security
USER nonroot:nonroot

# Set working directory
WORKDIR /app

# Health check - use exec form for distroless
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/k6-mcp", "--help"]

# Expose no ports as this is a stdio-based MCP server

# Run the MCP server
ENTRYPOINT ["/usr/local/bin/k6-mcp"]