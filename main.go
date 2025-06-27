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
	"github.com/oleiade/k6-mcp/internal/runner"
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

	// Register the run tool
	runTool := mcp.NewTool(
		"run",
		mcp.WithDescription("Run a k6 performance test with configurable parameters"),
		mcp.WithString(
			"script",
			mcp.Required(),
			mcp.Description("The k6 script content to run (JavaScript/TypeScript)"),
		),
		mcp.WithNumber(
			"vus",
			mcp.Description("Number of virtual users (default: 1, max: 50)"),
		),
		mcp.WithString(
			"duration",
			mcp.Description("Test duration (default: '30s', max: '5m')"),
		),
		mcp.WithNumber(
			"iterations",
			mcp.Description("Number of iterations per VU (overrides duration)"),
		),
		mcp.WithObject(
			"stages",
			mcp.Description("Load profile stages for ramping (array of {duration, target})"),
		),
		mcp.WithObject(
			"options",
			mcp.Description("Additional k6 options as JSON object"),
		),
	)

	s.AddTool(runTool, handleRun)

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

// handleRun handles the run tool requests.
func handleRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// Parse run options from arguments
	options, err := parseRunOptions(args)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	// Run the k6 test
	result, err := runner.RunK6Test(ctx, script, options)
	if err != nil {
		log.Printf("Run error: %v", err)
		// Return the run result even if there was an error
		// The result will contain error details for the client
	}

	// Convert result to JSON for structured response
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to serialize run result"), err
	}

	// Return structured result
	if result.Success {
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	// For failed tests, still return the result with details
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// parseRunOptions parses run options from the tool arguments.
func parseRunOptions(args map[string]interface{}) (*runner.RunOptions, error) {
	options := &runner.RunOptions{}

	// Parse VUs
	if vusValue, exists := args["vus"]; exists {
		if vus, ok := vusValue.(float64); ok {
			options.VUs = int(vus)
		} else {
			return nil, fmt.Errorf("vus must be a number")
		}
	}

	// Parse duration
	if durationValue, exists := args["duration"]; exists {
		if duration, ok := durationValue.(string); ok {
			options.Duration = duration
		} else {
			return nil, fmt.Errorf("duration must be a string")
		}
	}

	// Parse iterations
	if iterationsValue, exists := args["iterations"]; exists {
		if iterations, ok := iterationsValue.(float64); ok {
			options.Iterations = int(iterations)
		} else {
			return nil, fmt.Errorf("iterations must be a number")
		}
	}

	// Parse stages
	if stagesValue, exists := args["stages"]; exists {
		stagesData, err := json.Marshal(stagesValue)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal stages: %w", err)
		}
		
		var stages []runner.Stage
		if err := json.Unmarshal(stagesData, &stages); err != nil {
			return nil, fmt.Errorf("invalid stages format: %w", err)
		}
		options.Stages = stages
	}

	// Parse additional options
	if optionsValue, exists := args["options"]; exists {
		if opts, ok := optionsValue.(map[string]interface{}); ok {
			options.Options = opts
		} else {
			return nil, fmt.Errorf("options must be an object")
		}
	}

	return options, nil
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
