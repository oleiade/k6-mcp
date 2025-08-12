# k6-mcp

An **experimental** MCP (Model Context Protocol) server for k6, built in Go. It provides script validation, test execution, fast full‑text documentation search backed by an embedded SQLite FTS5 index, and intelligent script generation.

## Features

- **Script Validation**: Validates k6 scripts by executing them with minimal configuration (1 VU, 1 iteration)
- **Test Execution**: Runs k6 performance tests with configurable parameters (VUs, duration, stages, options)
- **Documentation Search (default)**: Fast full‑text search via SQLite FTS5 over the official k6 docs (embedded index)
- **Script Generation**: AI-powered k6 script generation with best practices and templates
- **Best Practices Resources**: Access to comprehensive k6 scripting guidelines and patterns
- **Security**: Comprehensive security measures including input size limits, dangerous pattern detection, and secure temporary file handling

## Quick Start

### Prerequisites

Install the following:

- **Go 1.24.4+**: For building and running the MCP server
- **k6**: Must be installed and available in PATH for script execution
- **Just**: Command runner for development tasks (recommended)

Install `just`:
```bash
# Just gives access to management commands for the project 
brew install just
```

### Quick Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/oleiade/k6-mcp
   cd k6-mcp
   ```

2. **Install the k6 MCP server** (generates the documentation index database, and installs `k6-mcp`):
   ```bash
   just install
   ```

Alternatively, to run without installing system‑wide:
```bash
just run
```

### Editor Integration

#### Cursor IDE

To use this MCP server with Cursor IDE:

1. **Ensure the MCP server is installed** (from step 3 above)

2. **Configure MCP settings**:
   Create or update your MCP configuration file (`~/.cursor/mcp_servers.json` or your editor's MCP config):

   ```json
   {
     "mcpServers": {
       "k6-mcp": {
         "command": "k6-mcp",
         "env": {}
       }
     }
   }
   ```

3. **Restart Cursor** or reload the MCP configuration.

4. **Use the k6 tools** in your Cursor chat:
   - Ask it to validate k6 scripts
   - Request performance test execution
   - Search k6 documentation
   - Generate k6 scripts from requirements

#### Claude Code

For Claude Code, use the following command to add the k6 MCP server to your configuration:

```bash
claude mcp add --scope=user --transport=stdio k6 k6-mcp
```

Note that this will add the k6 MCP server to your claude code user configuration. If you with to limit the perimeter of the tool to your project, set the scope option to `local` instead. 


#### Claude Desktop

For Claude Desktop, add the following to your MCP configuration:

```json
{
  "mcpServers": {
    "k6-mcp": {
      "command": "k6-mcp",
      "env": {}
    }
  }
}
```


## Available Tools

### validate_script

Validate a k6 script by running it with minimal configuration (1 VU, 1 iteration).

Parameters:
- `script` (string, required)

Returns: `valid`, `exit_code`, `stdout`, `stderr`, `error`, `duration`

### run_test

Run k6 performance tests with configurable parameters.

Parameters:
- `script` (string, required)
- `vus` (number, optional)
- `duration` (string, optional)
- `iterations` (number, optional)
- `stages` (object, optional)
- `options` (object, optional)

Returns: `success`, `exit_code`, `stdout`, `stderr`, `error`, `duration`, `metrics`, `summary`

### search_documentation

Full‑text search over the embedded k6 docs index (SQLite FTS5).

Parameters:
- `keywords` (string, required): FTS5 query string
- `max_results` (number, optional, default 10, max 20)

FTS5 tips:
- Space‑separated words imply AND: `checks thresholds` → `checks AND thresholds`
- Quotes for exact phrases: `"load testing"`
- Operators supported: `AND`, `OR`, `NEAR`, parentheses, prefix `http*`

Returns an array of results with `title`, `content`, `path`.

## Available Resources

### Best Practices Guide

Access comprehensive k6 scripting best practices covering:
- Test structure and organization
- Performance optimization techniques
- Error handling and validation patterns
- Authentication and security practices
- Browser testing guidelines
- Modern k6 features and protocols

**Resource URI:** `docs://k6/best_practices`

### Script Generation Template

AI-powered k6 script generation with structured workflow:
- Research and discovery phase
- Best practices integration
- Production-ready script creation
- Automated validation and testing
- File system integration

**Resource URI:** `prompts://k6/generate_script`


## Development

### Just Commands (Recommended)

```bash
# Build and run the MCP server (generates the SQLite index if missing)
just run

# Build binary locally (generates the SQLite index if missing)
just build

# Install into your Go bin (generates the SQLite index if missing)
just install

# Optimized release build (stripped, reproducible paths)
just release

# (Re)generate the embedded SQLite docs index
just index

# Optional (experimental embeddings): start vector DBs / helpers
just chroma
just milvus
just ingest
just verify
just reset
```

### Manual Commands

If you prefer not to use `just`:

```bash
# 1) Generate the SQLite FTS5 docs index (required for build/run because it is embedded)
go run -tags fts5 ./cmd/indexer

# 2) Start the MCP server
go run -tags fts5 ./cmd/k6-mcp

# Build a local binary
go build -tags fts5 -o k6-mcp ./cmd/k6-mcp

# Release‑style build (macOS example)
CGO_ENABLED=1 go build -tags 'fts5 sqlite_fts5' -trimpath -ldflags '-s -w' -o k6-mcp ./cmd/k6-mcp

# Run tests
go test ./...

# Lint
golangci-lint run
```

### Project Structure

```
├── cmd/
│   ├── k6-mcp/               # MCP server entry point
│   └── indexer/              # Builds the SQLite FTS5 docs index into dist/index.db
├── dist/
│   └── index.db              # Embedded SQLite FTS5 index (generated)
├── internal/
│   ├── runner/               # Test execution engine
│   ├── search/               # Full‑text search and indexer
│   ├── security/             # Security utilities
│   └── validator/            # Script validation
├── resources/                # MCP resources
│   ├── practices/            # Best practices guide
│   └── prompts/              # AI prompt templates
├── python-services/          # Optional utilities (embeddings, verification)
└── k6/scripts/               # Generated k6 scripts
```

## Security

The MCP server implements comprehensive security measures:

- **Input validation**: Size limits (1MB maximum) and dangerous pattern detection
- **Secure execution**: Blocks Node.js modules, system access, and malicious code patterns
- **File handling**: Restricted permissions (0600) and secure temporary file management
- **Resource limits**: Command execution timeouts (30s validation, 5m tests), max 50 VUs
- **Environment isolation**: Minimal k6 execution environment with proper cleanup
- **Docker hardening**: Non-root user, read-only filesystem, no new privileges

## Usage Examples

### Basic Script Validation

```bash
# In your MCP-enabled editor, ask:
"Can you validate this k6 script?"

# Then provide your k6 script content
```

### Performance Testing

```bash
# In your MCP-enabled editor, ask:
"Run a load test with 10 VUs for 2 minutes using this script"

# The system will execute the test and provide detailed metrics
```

### Documentation Search

```bash
# In your MCP-enabled editor, ask:
"Search for k6 authentication examples"
"How do I use thresholds in k6?"
"Show me WebSocket testing patterns"
```

### Script Generation

```bash
# In your MCP-enabled editor, ask:
"Generate a k6 script to test a REST API with authentication"
"Create a browser test for an e-commerce checkout flow"
"Generate a WebSocket load test script"
```

## Troubleshooting

### Build fails with “dist/index.db: no matching files”
Generate the docs index first:
```bash
just index
```

### Search returns no results
- Ensure the index exists: `ls dist/index.db`
- Rebuild the index: `just index`
- Try simpler queries, or quote phrases: `"load testing"`

### MCP Server Not Found
If your editor can't find the k6-mcp server:
1. Ensure it's installed: `just install`
2. Check your editor's MCP configuration
3. Verify the server starts: `k6-mcp` (should show MCP server output)

### Test Execution Failures
If k6 tests fail to execute:
1. Verify k6 is installed: `k6 version`
2. Check script syntax with the validate tool first
3. Ensure resources don't exceed limits (50 VUs, 5m duration)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Run tests: `go test ./...`
4. Run linter: `golangci-lint run`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
