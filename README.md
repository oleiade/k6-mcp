# k6-mcp

An experimental MCP (Model Context Protocol) server for k6, built in Go. This server provides k6 script validation capabilities through the MCP protocol.

## Features

- **Script Validation**: Validates k6 scripts by executing them with minimal configuration (1 VU, 1 iteration)
- **Security**: Comprehensive security measures including input size limits, dangerous pattern detection, and secure temporary file handling
- **Docker Support**: Ready-to-use Docker image with security best practices

## Quick Start

## Editor Integration

### Cursor IDE

To use this MCP server with Cursor IDE:

1. **Ensure Docker image is available**:
   ```bash
   docker build -t k6-mcp:latest .
   ```

2. **Configure MCP settings**:
   Create or update your MCP configuration file (`~/.cursor/mcp_servers.json` or your editor's MCP config):

   ```json
   {
     "mcpServers": {
       "k6-mcp": {
         "command": "docker",
         "args": [
           "run", "--rm", "-i",
           "--security-opt", "no-new-privileges:true",
           "--read-only",
           "--tmpfs", "/tmp:noexec,nosuid,size=50m",
           "--user", "65532:65532",
           "k6-mcp:latest"
         ],
         "env": {}
       }
     }
   }
   ```

3. **Restart Cursor** or reload the MCP configuration.

4. **Use the k6 validation tool** in your Cursor chat by asking it to validate k6 scripts.

### Other MCP-Compatible Editors

The same Docker configuration can be adapted for other MCP-compatible editors like Claude Desktop or other tools that support MCP. Adjust the configuration file path and format according to your editor's requirements.


### Using Go directly

1. **Install dependencies**:
   ```bash
   go mod tidy
   ```

2. **Run the server**:
   ```bash
   go run main.go
   ```

## Available Tools

### validate

Validates k6 scripts by executing them with minimal configuration.

**Parameters:**
- `script` (string, required): The k6 script content to validate (JavaScript/TypeScript)

**Returns:**
- `valid` (boolean): Whether the script is valid
- `exit_code` (integer): k6 exit code
- `stdout` (string): Standard output from k6
- `stderr` (string): Standard error from k6  
- `error` (string): Error message if validation failed
- `duration` (string): Time taken for validation

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
