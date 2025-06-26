// Package main provides the k6 MCP server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/oleiade/k6-mcp/internal/docs"
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

	// Register documentation resources
	docsPath := "k6-docs"
	docsHandler := docs.NewHandler(docsPath)
	
	// Register each document as a separate resource
	if err := registerDocumentationResources(s, docsHandler); err != nil {
		log.Printf("Warning: failed to register documentation resources: %v", err)
	}

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

// registerDocumentationResources registers all k6 documentation as individual MCP resources.
func registerDocumentationResources(s *server.MCPServer, handler *docs.Handler) error {
	ctx := context.Background()
	listResult, err := handler.ListResources(ctx, mcp.ListResourcesRequest{})
	if err != nil {
		return fmt.Errorf("failed to list documentation resources: %w", err)
	}

	for _, resource := range listResult.Resources {
		// Create resource handler for this specific resource
		resourceHandler := func(uri string) func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				// Override the request URI to ensure we get the right document
				request.Params.URI = uri
				result, err := handler.ReadResource(ctx, request)
				if err != nil {
					return nil, fmt.Errorf("failed to read resource: %w", err)
				}
				return result.Contents, nil
			}
		}(resource.URI)

		s.AddResource(resource, resourceHandler)
	}

	return nil
}
