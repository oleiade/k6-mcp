// Package main provides the k6 MCP server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/oleiade/k6-mcp/internal/logging"
	"github.com/oleiade/k6-mcp/internal/runner"
	"github.com/oleiade/k6-mcp/internal/search"
	"github.com/oleiade/k6-mcp/internal/validator"
)

const (
	maxResultsLimit = 20
)

func main() {
	logger := logging.Default()
	
	logger.Info("Starting k6 MCP server",
		slog.String("version", "1.0.0"),
		slog.Bool("resource_capabilities", true),
	)

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

	// Register the search tool
	searchTool := mcp.NewTool(
		"search",
		mcp.WithDescription("Search the k6 documentation for relevant information based on a query"),
		mcp.WithString(
			"query",
			mcp.Required(),
			mcp.Description("The search query to find relevant k6 documentation"),
		),
		mcp.WithNumber(
			"max_results",
			mcp.Description("Maximum number of results to return (default: 5, max: 20)"),
		),
	)

	s.AddTool(searchTool, handleSearch)

	logger.Info("Starting MCP server on stdio")
	
	if err := server.ServeStdio(s); err != nil {
		logger.Error("Server error", slog.String("error", err.Error()))
		panic(err)
	}
}

// handleValidate handles the validate tool requests.
func handleValidate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Add request correlation ID
	requestID := uuid.New().String()
	ctx = logging.ContextWithRequestID(ctx, requestID)
	startTime := time.Now()
	
	args := request.GetArguments()
	
	// Log request start
	logging.RequestStart(ctx, "validate", args)

	// Extract script content from arguments
	scriptValue, exists := args["script"]
	if !exists {
		err := fmt.Errorf("missing required parameter: script")
		logging.RequestEnd(ctx, "validate", false, time.Since(startTime), err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	script, ok := scriptValue.(string)
	if !ok {
		err := fmt.Errorf("script parameter must be a string")
		logging.RequestEnd(ctx, "validate", false, time.Since(startTime), err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Validate the k6 script
	result, err := validator.ValidateK6Script(ctx, script)
	if err != nil {
		logging.WithContext(ctx).Error("Validation processing error", 
			slog.String("error", err.Error()),
			slog.String("error_type", "validation_error"),
		)
		// Return the validation result even if there was an error
		// The result will contain error details for the client
	}

	// Convert result to JSON for structured response
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		logging.RequestEnd(ctx, "validate", false, time.Since(startTime), err)
		return mcp.NewToolResultError("failed to serialize validation result"), err
	}

	// Log request completion
	success := result != nil && result.Valid
	logging.RequestEnd(ctx, "validate", success, time.Since(startTime), nil)

	// Return structured result
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// handleRun handles the run tool requests.
func handleRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Add request correlation ID
	requestID := uuid.New().String()
	ctx = logging.ContextWithRequestID(ctx, requestID)
	startTime := time.Now()
	
	args := request.GetArguments()
	
	// Log request start
	logging.RequestStart(ctx, "run", args)

	// Extract script content from arguments
	scriptValue, exists := args["script"]
	if !exists {
		err := fmt.Errorf("missing required parameter: script")
		logging.RequestEnd(ctx, "run", false, time.Since(startTime), err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	script, ok := scriptValue.(string)
	if !ok {
		err := fmt.Errorf("script parameter must be a string")
		logging.RequestEnd(ctx, "run", false, time.Since(startTime), err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Parse run options from arguments
	options, err := parseRunOptions(args)
	if err != nil {
		logging.RequestEnd(ctx, "run", false, time.Since(startTime), err)
		return mcp.NewToolResultError(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	// Run the k6 test
	result, err := runner.RunK6Test(ctx, script, options)
	if err != nil {
		logging.WithContext(ctx).Error("Run processing error", 
			slog.String("error", err.Error()),
			slog.String("error_type", "run_error"),
		)
		// Return the run result even if there was an error
		// The result will contain error details for the client
	}

	// Convert result to JSON for structured response
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		logging.RequestEnd(ctx, "run", false, time.Since(startTime), err)
		return mcp.NewToolResultError("failed to serialize run result"), err
	}

	// Log request completion
	success := result != nil && result.Success
	logging.RequestEnd(ctx, "run", success, time.Since(startTime), nil)

	// Return structured result
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

// handleSearch handles the search tool requests.
func handleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Add request correlation ID
	requestID := uuid.New().String()
	ctx = logging.ContextWithRequestID(ctx, requestID)
	startTime := time.Now()
	
	args := request.GetArguments()
	
	// Log request start
	logging.RequestStart(ctx, "search", args)

	// Extract query from arguments
	queryValue, exists := args["query"]
	if !exists {
		err := fmt.Errorf("missing required parameter: query")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	query, ok := queryValue.(string)
	if !ok {
		err := fmt.Errorf("query parameter must be a string")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	if query == "" {
		err := fmt.Errorf("query parameter cannot be empty")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Parse search options
	options := search.DefaultOptions()

	// Parse max_results if provided
	if maxResultsValue, exists := args["max_results"]; exists {
		if maxResults, ok := maxResultsValue.(float64); ok {
			maxResultsInt := int(maxResults)
			// Enforce reasonable limits
			if maxResultsInt <= 0 {
				maxResultsInt = 5
			} else if maxResultsInt > maxResultsLimit {
				maxResultsInt = maxResultsLimit
			}
			options.MaxResults = maxResultsInt
		} else {
			err := fmt.Errorf("max_results must be a number")
			logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	// Create search client and perform search
	client, err := search.NewSearch(search.BackendChroma, options)
	if err != nil {
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to create search client: %v", err)), nil
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			logging.WithContext(ctx).Warn("Failed to close search client", 
				slog.String("error", closeErr.Error()),
			)
		}
	}()

	results, err := client.Search(ctx, query)
	if err != nil {
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	// Log search results
	logging.SearchEvent(ctx, query, len(results), time.Since(startTime), nil)

	// Create response structure
	response := struct {
		Query       string          `json:"query"`
		Results     []search.Result `json:"results"`
		ResultCount int             `json:"result_count"`
	}{
		Query:       query,
		Results:     results,
		ResultCount: len(results),
	}

	// Convert result to JSON for structured response
	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError("failed to serialize search results"), err
	}

	// Log request completion
	logging.RequestEnd(ctx, "search", true, time.Since(startTime), nil)

	return mcp.NewToolResultText(string(resultJSON)), nil
}
