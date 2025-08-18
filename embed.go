package k6mcp

import (
	"embed"
	_ "embed"
)

//go:embed dist/index.db
var EmbeddedDB []byte

//go:embed dist/definitions/types/k6/**
var TypeDefinitions embed.FS

//go:embed resources/**
var Resources embed.FS
