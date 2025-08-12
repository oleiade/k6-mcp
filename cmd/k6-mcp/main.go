//go:build fts5

// Package main provides the k6 MCP server.
package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	k6mcp "github.com/oleiade/k6-mcp"
	"github.com/oleiade/k6-mcp/internal/handlers"
	"github.com/oleiade/k6-mcp/internal/logging"
	"github.com/oleiade/k6-mcp/internal/runner"

	"github.com/oleiade/k6-mcp/internal/validator"
)

func main() {
	logger := logging.Default()

	logger.Info("Starting k6 MCP server",
		slog.String("version", "1.0.0"),
		slog.Bool("resource_capabilities", true),
	)

	// Open the embedded database SQLite file
	db, dbFile, err := openDB(k6mcp.EmbeddedDB)
	if err != nil {
		logger.Error("Error opening database", "error", err)
		panic(err)
	}
	defer closeDB(logger, db)
	defer removeDBFile(logger, dbFile)

	s := server.NewMCPServer(
		"k6",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
	)

	// Register tools
	registerRunTool(s, handlers.NewRunHandler())
	registerDocumentationTools(s, handlers.NewFullTextSearchHandler(db))
	registerValidationTool(s, handlers.NewValidationHandler())

	// Register resources
	registerBestPracticesResource(s)

	// Register prompts
	registerGenerateScriptPrompt(s, handlers.NewScriptGenerator())

	logger.Info("Starting MCP server on stdio")
	if err := server.ServeStdio(s); err != nil {
		logger.Error("Server error", slog.String("error", err.Error()))
		return
	}
}

func registerRunTool(s *server.MCPServer, h handlers.ToolHandler) {
	// Register the run tool
	runTool := mcp.NewTool(
		"run_test",
		mcp.WithDescription("Run a k6 performance test with configurable parameters. Returns detailed execution results including performance metrics, failure analysis, and optimization recommendations."),
		mcp.WithString(
			"script",
			mcp.Required(),
			mcp.Description("The k6 script content to run (JavaScript/TypeScript). Should be a valid k6 script with proper imports and default function."),
		),
		mcp.WithNumber(
			"vus",
			mcp.Description("Number of virtual users (default: 1, max: 50). Examples: 1 for basic test, 10 for moderate load, 50 for stress test."),
		),
		mcp.WithString(
			"duration",
			mcp.Description("Test duration (default: '30s', max: '5m'). Examples: '30s', '2m', '5m'. Overridden by iterations if specified."),
		),
		mcp.WithNumber(
			"iterations",
			mcp.Description("Number of iterations per VU (overrides duration). Examples: 1 for single run, 100 for throughput test."),
		),
		mcp.WithObject(
			"stages",
			mcp.Description("Load profile stages for ramping (array of {duration, target}). Example: [{\"duration\": \"30s\", \"target\": 10}, {\"duration\": \"1m\", \"target\": 20}]"),
		),
		mcp.WithObject(
			"options",
			mcp.Description("Additional k6 options as JSON object. Example: {\"thresholds\": {\"http_req_duration\": [\"p(95)<500\"]}}"),
		),
	)

	s.AddTool(runTool, h.Handle)
}

func registerDocumentationTools(s *server.MCPServer, h handlers.ToolHandler) {
	// Register the search tool
	searchTool := mcp.NewTool(
		"search_documentation",
		mcp.WithDescription("Search up-to-date k6 documentation using SQLite FTS5 full-text search. Use proactively while authoring or validating scripts to find best practices, troubleshoot errors, discover examples/templates, and learn idiomatic k6 usage. Query semantics: space-separated terms are ANDed by default; use quotes for exact phrases; FTS5 operators (AND, OR, NEAR, parentheses) and prefix wildcards (e.g., http*) are supported. Returns structured results with title, content, and path."),
		mcp.WithString(
			"keywords",
			mcp.Required(),
			mcp.Description("FTS5 query string. Use space-separated terms (implicit AND), quotes for exact phrases, and optional FTS5 operators. Examples: 'load' → matches load; 'load testing' → matches load AND testing; '\"load testing\"' → exact phrase; 'thresholds OR checks'; 'stages NEAR/5 ramping'; 'http*' for prefix."),
		),
		mcp.WithNumber(
			"max_results",
			mcp.Description("Maximum number of results to return (default: 10, max: 20). Use 5–10 for focused results, 15–20 for broader coverage."),
		),
	)

	s.AddTool(searchTool, h.Handle)
}

func registerValidationTool(s *server.MCPServer, h handlers.ToolHandler) {
	// Register the validate tool
	validateTool := mcp.NewTool(
		"validate_script",
		mcp.WithDescription("Validate a k6 script by running it with minimal configuration (1 VU, 1 iteration). Returns detailed validation results with syntax errors, runtime issues, and actionable recommendations for fixing problems."),
		mcp.WithString(
			"script",
			mcp.Required(),
			mcp.Description("The k6 script content to validate (JavaScript/TypeScript). Example: 'import http from \"k6/http\"; export default function() { http.get(\"https://httpbin.org/get\"); }'"),
		),
	)

	s.AddTool(validateTool, h.Handle)
}

func registerBestPracticesResource(s *server.MCPServer) {
	bestPracticesResource := mcp.NewResource(
		"docs://k6/best_practices",
		"k6 best practices",
		mcp.WithResourceDescription("Provides a list of best practices for writing k6 scripts."),
		mcp.WithMIMEType("text/markdown"),
	)

	s.AddResource(bestPracticesResource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		content, err := os.ReadFile("resources/practices/PRACTICES.md")
		if err != nil {
			return nil, err
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "docs://k6/best_practices",
				MIMEType: "text/markdown",
				Text:     string(content),
			},
		}, nil
	})
}

func registerGenerateScriptPrompt(s *server.MCPServer, h handlers.PromptHandler) {
	generateScriptPrompt := mcp.NewPrompt(
		"generate_script",
		mcp.WithPromptDescription("Generate a k6 script based on the user's request."),
		mcp.WithArgument("description", mcp.ArgumentDescription("The description of the script to generate.")),
	)

	s.AddPrompt(generateScriptPrompt, h.Handle)
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
		return mcp.NewToolResultError("Missing required parameter 'script'. Please provide your k6 script content as a string. Example: {\"script\": \"import http from 'k6/http'; export default function() { http.get('https://httpbin.org/get'); }\"}"), nil
	}

	script, ok := scriptValue.(string)
	if !ok {
		err := fmt.Errorf("script parameter must be a string")
		logging.RequestEnd(ctx, "validate", false, time.Since(startTime), err)
		return mcp.NewToolResultError("Parameter 'script' must be a string containing your k6 script code. Received: " + fmt.Sprintf("%T", scriptValue)), nil
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

//// handleGenerateScript handles the generate_script prompt requests.
//func handleGenerateScript(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
//	// Add request correlation ID
//	requestID := uuid.New().String()
//	ctx = logging.ContextWithRequestID(ctx, requestID)
//	startTime := time.Now()
//
//	args := request.Params.Arguments
//
//	// Convert args to interface{} map for logging
//	logArgs := make(map[string]interface{})
//	for k, v := range args {
//		logArgs[k] = v
//	}
//
//	// Log request start
//	logging.RequestStart(ctx, "generate_script", logArgs)
//
//	// Extract description from arguments
//	description, exists := args["description"]
//	if !exists {
//		err := fmt.Errorf("missing required parameter: description")
//		logging.RequestEnd(ctx, "generate_script", false, time.Since(startTime), err)
//		return nil, fmt.Errorf("missing required parameter 'description'. Please provide a description of the k6 script you want to generate")
//	}
//
//	if description == "" {
//		err := fmt.Errorf("description parameter cannot be empty")
//		logging.RequestEnd(ctx, "generate_script", false, time.Since(startTime), err)
//		return nil, fmt.Errorf("description parameter cannot be empty. Please provide a detailed description of the k6 script you want to generate")
//	}
//
//	// Load prompt template from file or embedded content
//	templateContent, err := loadPromptTemplate("resources/prompts/generate_script.md", generateScriptTemplate)
//	if err != nil {
//		logging.RequestEnd(ctx, "generate_script", false, time.Since(startTime), err)
//		return nil, fmt.Errorf("failed to load prompt template: %w", err)
//	}
//
//	// Replace template variables
//	promptText := strings.Replace(templateContent, "{{.Description}}", description, 1)
//
//	result := mcp.NewGetPromptResult(
//		"A k6 script",
//		[]mcp.PromptMessage{
//			mcp.NewPromptMessage(
//				mcp.RoleAssistant,
//				mcp.NewTextContent(promptText),
//			),
//		},
//	)
//
//	// Log request completion
//	logging.RequestEnd(ctx, "generate_script", true, time.Since(startTime), nil)
//
//	return result, nil
//}

// getPresetConfigurations returns predefined test configurations
func getPresetConfigurations() map[string]runner.RunOptions {
	return map[string]runner.RunOptions{
		"smoke_test": {
			VUs:      1,
			Duration: "30s",
		},
		"load_test": {
			VUs:      10,
			Duration: "5m",
		},
		"stress_test": {
			VUs:      50,
			Duration: "10m",
		},
		"spike_test": {
			Stages: []runner.Stage{
				{Duration: "2m", Target: 1},
				{Duration: "30s", Target: 50},
				{Duration: "2m", Target: 50},
				{Duration: "30s", Target: 1},
				{Duration: "2m", Target: 1},
			},
		},
		"ramp_up_test": {
			Stages: []runner.Stage{
				{Duration: "1m", Target: 5},
				{Duration: "2m", Target: 10},
				{Duration: "2m", Target: 15},
				{Duration: "1m", Target: 0},
			},
		},
		"endurance_test": {
			VUs:      10,
			Duration: "30m",
		},
		"quick_test": {
			VUs:        1,
			Iterations: 1,
		},
		"api_test": {
			VUs:      5,
			Duration: "2m",
		},
	}
}

// openDB loads the database file from the embedded data, writes it to a temporary file,
// and returns the file handle and a database connection.
//
// The caller is responsible for closing the database connection and removing the temporary file.
func openDB(dbData []byte) (db *sql.DB, dbFile *os.File, err error) {
	// Load the search index database file from the embedded data
	dbFile, err = os.CreateTemp("", "k6-mcp-index-*.db")
	if err != nil {
		return nil, nil, fmt.Errorf("error creating temporary database file: %w", err)
	}

	_, err = dbFile.Write(dbData)
	if err != nil {
		return nil, nil, fmt.Errorf("error writing index database to temporary file: %w", err)
	}
	err = dbFile.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("error closing temporary database file: %w", err)
	}

	// Open SQLite connection
	db, err = sql.Open("sqlite3", dbFile.Name()+"?mode=ro")
	if err != nil {
		return nil, nil, fmt.Errorf("error opening temporary database file: %w", err)
	}

	return db, dbFile, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func removeDBFile(logger *slog.Logger, dbFile *os.File) {
	err := os.Remove(dbFile.Name())
	if err != nil {
		logger.Error("Error removing temporary database file", "error", err)
	}
}

func closeDB(logger *slog.Logger, db *sql.DB) {
	err := db.Close()
	if err != nil {
		logger.Error("Error closing database connection", "error", err)
	}
}
