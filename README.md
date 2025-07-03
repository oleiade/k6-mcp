# k6-mcp

An experimental MCP (Model Context Protocol) server for k6, built in Go. This server provides comprehensive k6 performance testing capabilities through the MCP protocol, including script validation, test execution, documentation search, and intelligent script generation.

## Features

- **Script Validation**: Validates k6 scripts by executing them with minimal configuration (1 VU, 1 iteration)
- **Test Execution**: Runs k6 performance tests with configurable parameters (VUs, duration, stages, options)
- **Documentation Search**: Semantic search through k6 documentation using vector database
- **Script Generation**: AI-powered k6 script generation with best practices and templates
- **Best Practices Resources**: Access to comprehensive k6 scripting guidelines and patterns
- **Security**: Comprehensive security measures including input size limits, dangerous pattern detection, and secure temporary file handling

## Quick Start

### Prerequisites

Before using k6-mcp, ensure you have the following installed:

- **Go 1.24.4+**: For building and running the MCP server
- **k6**: Must be installed and available in PATH for script execution
- **Docker**: For running the vector database (required for search functionality)
- **Just**: Command runner for development tasks (recommended)

Install `just`, and `uv` for the best experience:
```bash
# Just gives access to management commands for the project 
brew install just

# UV is used to facilitate the execution and requirements management
# of our python scripts
brew install uv
```

### Quick Setup

1. **Clone the repository** (with submodules for k6 documentation):
   ```bash
   git clone --recursive https://github.com/oleiade/k6-mcp
   cd k6-mcp
   ```

2. **Initialize the project** (starts ChromaDB and ingests k6 documentation):
   ```bash
   just initialize
   ```

3. **Install the MCP server**:
   ```bash
   go install github.com/oleiade/k6-mcp/cmd/k6-mcp@main
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

### validate

Validates k6 scripts by executing them with minimal configuration (1 VU, 1 iteration).

**Parameters:**
- `script` (string, required): The k6 script content to validate (JavaScript/TypeScript)

**Returns:**
- `valid` (boolean): Whether the script is valid
- `exit_code` (integer): k6 exit code
- `stdout` (string): Standard output from k6
- `stderr` (string): Standard error from k6  
- `error` (string): Error message if validation failed
- `duration` (string): Time taken for validation

### run

Runs k6 performance tests with configurable parameters for comprehensive load testing.

**Parameters:**
- `script` (string, required): The k6 script content to run (JavaScript/TypeScript)
- `vus` (number, optional): Number of virtual users (default: 1, max: 50)
- `duration` (string, optional): Test duration (default: '30s', max: '5m')
- `iterations` (number, optional): Number of iterations per VU (overrides duration)
- `stages` (object, optional): Load profile stages for ramping (array of {duration, target})
- `options` (object, optional): Additional k6 options as JSON object

**Returns:**
- `success` (boolean): Whether the test completed successfully
- `exit_code` (integer): k6 exit code
- `stdout` (string): Standard output from k6
- `stderr` (string): Standard error from k6
- `error` (string): Error message if test failed
- `duration` (string): Time taken for test execution
- `metrics` (object): Raw k6 metrics data
- `summary` (object): Test summary with key performance metrics:
  - `total_requests` (integer): Total HTTP requests made
  - `failed_requests` (integer): Number of failed requests
  - `avg_response_time_ms` (number): Average response time in milliseconds
  - `p95_response_time_ms` (number): 95th percentile response time in milliseconds
  - `request_rate_per_second` (number): Request rate per second
  - `data_received` (string): Amount of data received
  - `data_sent` (string): Amount of data sent

### search

Searches k6 documentation using semantic similarity via ChromaDB vector database.

**Parameters:**
- `query` (string, required): The search query to find relevant k6 documentation
- `max_results` (number, optional): Maximum number of results to return (default: 5, max: 20)

**Returns:**
- `query` (string): The original search query
- `results` (array): Array of search results, each containing:
  - `content` (string): Documentation content
  - `metadata` (object): Document metadata
  - `score` (number): Similarity score (0-1, higher is more relevant)
  - `source` (string): Source identifier
- `count` (integer): Number of results returned

**Prerequisites:**
- ChromaDB must be running (`just chroma`)
- k6 documentation must be ingested (`just ingest`)

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

The project uses `just` for common development tasks:

```bash
# Initialize submodules and start services
just initialize

# Run the MCP server
just run

# Start ChromaDB vector database
just chroma

# Ingest k6 documentation into vector database
just ingest

# Verify database ingestion
just verify

# Reset ChromaDB (clean slate)
just reset
```

### Manual Commands

If you prefer not to use `just`:

```bash
# Build the project
go build -o k6-mcp ./cmd/k6-mcp

# Run the MCP server
go run ./cmd/k6-mcp

# Run tests
go test ./...

# Run linter
golangci-lint run

# Start ChromaDB with Docker
docker compose -f docker-compose.chroma.yml up -d

# Ingest documentation
cd python-services && ./ingest.py
```

### Project Structure

```
├── cmd/k6-mcp/                # MCP server entry point
├── internal/                  # Internal packages
│   ├── runner/               # Test execution engine
│   ├── search/               # Documentation search
│   ├── security/             # Security utilities
│   └── validator/            # Script validation
├── k6-docs/                  # Git submodule: k6 documentation
├── resources/                # MCP resources
│   ├── practices/           # Best practices guide
│   └── prompts/             # AI prompt templates
├── python-services/          # Python utilities for ingestion
└── k6/scripts/              # Generated k6 scripts
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

### Search Not Working

If search functionality isn't working:
1. Ensure ChromaDB is running: `just chroma`
2. Verify documentation is ingested: `just ingest`
3. Check ingestion status: `just verify`

### MCP Server Not Found

If your editor can't find the k6-mcp server:
1. Ensure it's installed: `go install github.com/oleiade/k6-mcp/cmd/k6-mcp@main`
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
