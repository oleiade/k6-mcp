# k6-mcp

An experimental MCP (Model Context Protocol) server for k6, built in Go. This server provides k6 script validation capabilities through the MCP protocol.

## Features

- **Script Validation**: Validates k6 scripts by executing them with minimal configuration (1 VU, 1 iteration)
- **Security**: Comprehensive security measures including input size limits, dangerous pattern detection, and secure temporary file handling
- **Docker Support**: Ready-to-use Docker image with security best practices

## Quick Start

### Prerequisites ‼️

We recommend installing the `just` command to benefit from our `just` commands:
```bash
brew install just
```

With `just` installed, we can now run Chroma DB in the background, and ingest the k6 documentation into it to support the k6-mcp server's search feature.

```bash
# This will start a chromaDB container, and proceed with feeding it k6 documentation sources
just initialize
```

### Editor Integration

### Cursor IDE

To use this MCP server with Cursor IDE:

1. **Install the k6-mcp server**:

   ```bash
   go install github.com/oleiade/k6-mcp/cmd/k6-mcp@main
   ```


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

4. **Use the k6 validation tool** in your Cursor chat by asking it to validate k6 scripts.


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

Runs k6 performance tests with configurable parameters.

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

Searches k6 documentation using semantic similarity.

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


## Development

### Building

```bash
go build -o k6-mcp main.go
```

### Testing

```bash
go test ./...
```

### Linting

```bash
golangci-lint run
```

## Docker Compose

For development with Docker Compose:

```bash
# Start the service
docker-compose up -d

# Check logs
docker-compose logs -f

# Stop the service
docker-compose down
```

## Security

The MCP server implements several security measures:

- Input size limits (1MB maximum)
- Dangerous pattern detection (blocks Node.js modules, system access)
- Secure temporary file handling with restricted permissions (0600)
- Command execution timeouts (30s default)
- Minimal environment for k6 execution
- Proper cleanup of temporary files
- Docker security hardening (non-root user, read-only filesystem, no new privileges)
