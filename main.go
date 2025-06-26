// Package main provides the k6 MCP server.
package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/oleiade/k6-mcp/internal/validator"
)

func main() {
	s := server.NewMCPServer(
		"k6",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
	)

	// Register the validate tool
	validateTool := mcp.NewTool(
		"validate",
		mcp.WithDescription("Validate a k6 script by running it with minimal configuration (1 VU, 1 iteration)"),
		mcp.WithString(
			"script",
			mcp.Required(),
			mcp.Description("The k6 script content to validate (JavaScript/TypeScript)"),
		),
	)

	s.AddTool(validateTool, handleValidate)

	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}

// handleValidate handles the validate tool requests.
func handleValidate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	// Extract script content from arguments
	scriptValue, exists := args["script"]
	if !exists {
		return mcp.NewToolResultError("missing required parameter: script"), nil
	}

	script, ok := scriptValue.(string)
	if !ok {
		return mcp.NewToolResultError("script parameter must be a string"), nil
	}

	// Validate the k6 script
	result, err := validator.ValidateK6Script(ctx, script)
	if err != nil {
		log.Printf("Validation error: %v", err)
		// Return the validation result even if there was an error
		// The result will contain error details for the client
	}

	// Convert result to JSON for structured response
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to serialize validation result"), err
	}

	// Return structured result
	if result.Valid {
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	// For invalid scripts, still return the result
	return mcp.NewToolResultText(string(resultJSON)), nil
}
