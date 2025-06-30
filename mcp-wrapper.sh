#!/bin/bash
set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Change to the project directory
cd "$SCRIPT_DIR"

# Ensure the compose stack is running
docker compose up -d --wait

# Execute the MCP server
exec docker compose exec -T k6-mcp /usr/local/bin/k6-mcp "$@"