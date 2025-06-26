# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an experimental MCP (Model Context Protocol) server for k6, built in Go. The project uses the `mcp-go` library to create an MCP server that communicates via stdio.

## Architecture

- **main.go**: Entry point that creates and serves an MCP server with resource capabilities, logging, and recovery middleware
- **go.mod**: Defines the module as `github.com/oleiade/k6-mcp` using Go 1.24.4
- Uses `github.com/mark3labs/mcp-go` v0.32.0 as the core MCP library

## Common Commands

### Development
```bash
# Run the MCP server
go run main.go

# Build the project
go build

# Install dependencies
go mod tidy

# Update dependencies
go get -u ./...
```

### Testing
```bash
# Run tests (when available)
go test ./...

# Run tests with verbose output
go test -v ./...
```

### Code Quality
```bash
# Run golangci-lint (install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
golangci-lint run

# Run golangci-lint with auto-fix
golangci-lint run --fix

# Run specific linters
golangci-lint run --enable-only=gofmt,goimports
```

## Code Quality Requirements

**IMPORTANT**: All code must pass golangci-lint checks before being committed. The project uses a comprehensive linting configuration that enforces:

- Idiomatic Go practices following the Go team's standards
- Code style consistency with popular Go projects (Kubernetes, Docker, etc.)
- Security best practices via gosec
- Performance optimizations
- Proper error handling and context usage
- Documentation standards for exported functions

Always run `golangci-lint run` before committing changes. The configuration is optimized for Go 1.24+ and includes 40+ linters covering style, bugs, performance, and security.

## MCP Server Configuration

The server is configured with:
- Name: "k6"
- Version: "1.0.0"  
- Resource capabilities enabled (read/write)
- Logging middleware
- Recovery middleware
- Stdio transport for communication

## Available Tools

### validate
Validates k6 scripts by executing them with minimal configuration (1 VU, 1 iteration).

**Parameters:**
- `script` (string, required): The k6 script content to validate (JavaScript/TypeScript)

**Returns:**
- JSON object with validation results including:
  - `valid` (bool): Whether the script is valid
  - `exit_code` (int): k6 exit code
  - `stdout` (string): Standard output from k6
  - `stderr` (string): Standard error from k6
  - `error` (string): Error message if validation failed
  - `duration` (string): Time taken for validation

## Security Features

The implementation includes comprehensive security measures:
- Input size limits (1MB max)
- Dangerous pattern detection (blocks Node.js modules, system access)
- Secure temporary file handling with restricted permissions (0600)
- Command execution timeouts (30s default)
- Minimal environment for k6 execution
- Proper cleanup of temporary files

## Project Structure

```
├── main.go                    # MCP server entry point and tool registration
├── internal/
│   ├── validator/            # Core k6 validation logic
│   │   └── validator.go      # Script validation, temp file handling, k6 execution
│   └── security/             # Security utilities
│       └── security.go       # Input validation, dangerous pattern detection
├── test_scripts/             # Test k6 scripts for validation
└── .golangci.yml            # Comprehensive linting configuration
```