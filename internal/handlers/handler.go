package handlers

import (
	"context"
	_ "embed"
	"github.com/mark3labs/mcp-go/mcp"
)

// ToolHandler defines an interface for MCP tool calls handlers.
type ToolHandler interface {
	Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

type PromptHandler interface {
	Handle(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error)
}
