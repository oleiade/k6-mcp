package k6mcp

import _ "embed"

//go:embed dist/index.db
var EmbeddedDB []byte
