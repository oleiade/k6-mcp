package k6mcp

import (
	"embed"
	_ "embed"
)

//go:embed dist/index.db
var EmbeddedDB []byte

//go:embed resources/**
var Resources embed.FS
