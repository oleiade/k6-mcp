# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.24-alpine AS builder

# Install k6 build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /build

# Copy go modules files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o k6-mcp \
    ./main.go

# Final stage - distroless for minimal size
FROM gcr.io/distroless/static:nonroot

# Install k6 binary
COPY --from=grafana/k6:latest /usr/bin/k6 /usr/bin/k6

# Copy the binary from builder stage
COPY --from=builder /build/k6-mcp /usr/local/bin/k6-mcp

# Copy the k6 documentation
COPY --from=builder /build/k6-docs /app/k6-docs

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