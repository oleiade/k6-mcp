// Package main provides the k6 MCP server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
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
		mcp.WithDescription("Validate a k6 script by running it with minimal configuration (1 VU, 1 iteration). Returns detailed validation results with syntax errors, runtime issues, and actionable recommendations for fixing problems."),
		mcp.WithString(
			"script",
			mcp.Required(),
			mcp.Description("The k6 script content to validate (JavaScript/TypeScript). Example: 'import http from \"k6/http\"; export default function() { http.get(\"https://httpbin.org/get\"); }'"),
		),
	)

	s.AddTool(validateTool, handleValidate)

	// Register the run tool
	runTool := mcp.NewTool(
		"run",
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

	s.AddTool(runTool, handleRun)

	// Register the search tool
	searchTool := mcp.NewTool(
		"search",
		mcp.WithDescription("Search k6 documentation using semantic similarity. Use proactively when writing k6 scripts to ensure best practices, troubleshoot validation errors, find script examples and templates, learn k6 patterns, and discover performance testing techniques. Essential for avoiding common mistakes and following idiomatic k6 development workflows."),
		mcp.WithString(
			"query",
			mcp.Required(),
			mcp.Description("Search query for k6 documentation. Development workflow examples: 'script validation errors', 'threshold configuration best practices', 'HTTP authentication patterns', 'load testing stages setup'. Troubleshooting examples: 'debugging failed tests', 'common k6 syntax errors', 'performance optimization tips'. Feature learning: 'WebSocket testing', 'GraphQL API testing', 'browser automation', 'metrics and checks'."),
		),
		mcp.WithNumber(
			"max_results",
			mcp.Description("Maximum number of results to return (default: 5, max: 20). Use 5-10 for focused results, 15-20 for comprehensive coverage."),
		),
	)

	s.AddTool(searchTool, handleSearch)

	// In your mcp-go server setup
	bestPracticesResource := mcp.NewResource(
		"docs://k6/best_practices",
		"k6 best practices",
		mcp.WithResourceDescription("Provides a list of best practices for writing k6 scripts."),
		mcp.WithMIMEType("text/markdown"),
	)

	s.AddResource(bestPracticesResource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		content, err := os.ReadFile("README.md")
		if err != nil {
			return nil, err
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "docs://readme",
				MIMEType: "text/markdown",
				Text:     string(content),
			},
		}, nil
	})

	generateScriptPrompt := mcp.NewPrompt(
		"generate_script",
		mcp.WithPromptDescription("Generate a k6 script based on the user's request."),
		mcp.WithArgument("description", mcp.ArgumentDescription("The description of the script to generate.")),
	)

	s.AddPrompt(generateScriptPrompt, handleGenerateScript)

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
		return mcp.NewToolResultError("Missing required parameter 'script'. Please provide your k6 script content as a string. Tip: Use the 'validate' tool first to check your script before running."), nil
	}

	script, ok := scriptValue.(string)
	if !ok {
		err := fmt.Errorf("script parameter must be a string")
		logging.RequestEnd(ctx, "run", false, time.Since(startTime), err)
		return mcp.NewToolResultError("Parameter 'script' must be a string containing your k6 script code. Received: " + fmt.Sprintf("%T", scriptValue)), nil
	}

	// Parse run options from arguments
	options, err := parseRunOptions(args)
	if err != nil {
		// Include parameter suggestions in error message
		suggestions := suggestParameterImprovements(args)
		suggestionText := ""
		if len(suggestions) > 0 {
			suggestionText = " Suggestions: " + strings.Join(suggestions[:min(2, len(suggestions))], "; ")
		}
		logging.RequestEnd(ctx, "run", false, time.Since(startTime), err)
		return mcp.NewToolResultError(fmt.Sprintf("Invalid parameters: %v. Check parameter types and ranges.%s Use the 'search' tool with query 'run options' for more examples.", err, suggestionText)), nil
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

// handleGenerateScript handles the generate_script prompt requests.
func handleGenerateScript(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	// Add request correlation ID
	requestID := uuid.New().String()
	ctx = logging.ContextWithRequestID(ctx, requestID)
	startTime := time.Now()

	args := request.Params.Arguments

	// Convert args to interface{} map for logging
	logArgs := make(map[string]interface{})
	for k, v := range args {
		logArgs[k] = v
	}

	// Log request start
	logging.RequestStart(ctx, "generate_script", logArgs)

	// Extract description from arguments
	description, exists := args["description"]
	if !exists {
		err := fmt.Errorf("missing required parameter: description")
		logging.RequestEnd(ctx, "generate_script", false, time.Since(startTime), err)
		return nil, fmt.Errorf("missing required parameter 'description'. Please provide a description of the k6 script you want to generate")
	}

	if description == "" {
		err := fmt.Errorf("description parameter cannot be empty")
		logging.RequestEnd(ctx, "generate_script", false, time.Since(startTime), err)
		return nil, fmt.Errorf("description parameter cannot be empty. Please provide a detailed description of the k6 script you want to generate")
	}

	// This is the core of your guidance.
	promptText := fmt.Sprintf(`
            You are an expert k6 performance testing engineer. Your task is to generate a high-quality k6 script based on the following user request:

            **User Request:**
            %s

            To ensure the quality and accuracy of the script, you must follow these steps precisely:

            1.  **Consult the Documentation:** Before writing any code, use the "k6/search" tool to look up relevant functions and concepts based on the user's request. For example, if the user mentions "checking for a 200 status code," you should search for "k6 check status code."

            2.  **Review Best Practices:** After your initial research, you must read the k6 best practices by accessing the "docs://k6/best_practices" resource.

            3.  **Write the Script:** Now, write the k6 script. It is IMPORTANT that your script MUST adhere to the best practices you just reviewed. The script should be well-commented to explain the logic.

            4. **Validate the Script:** Validate the script using the "k6/validate" tool.

            5. **Final Review:** Before presenting the script, double-check that you have correctly implemented the user's request and followed the best practices.

			6. **Offer to run the script:** If the script is valid, offer to run it using the "k6/run" tool.
        `, description)

	result := mcp.NewGetPromptResult(
		"A k6 script",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleAssistant,
				mcp.NewTextContent(promptText),
			),
		},
	)

	// Log request completion
	logging.RequestEnd(ctx, "generate_script", true, time.Since(startTime), nil)

	return result, nil
}

// parseRunOptions parses run options from the tool arguments.
func parseRunOptions(args map[string]interface{}) (*runner.RunOptions, error) {
	options := &runner.RunOptions{}

	// Apply smart defaults based on context
	applySmartDefaults(options, args)

	// Parse VUs
	if vusValue, exists := args["vus"]; exists {
		if vus, ok := vusValue.(float64); ok {
			options.VUs = int(vus)
		} else {
			return nil, fmt.Errorf("vus must be a number (received %T). Example: 10", vusValue)
		}
	}

	// Parse duration
	if durationValue, exists := args["duration"]; exists {
		if duration, ok := durationValue.(string); ok {
			options.Duration = duration
		} else {
			return nil, fmt.Errorf("duration must be a string (received %T). Examples: '30s', '2m', '5m'", durationValue)
		}
	}

	// Parse iterations
	if iterationsValue, exists := args["iterations"]; exists {
		if iterations, ok := iterationsValue.(float64); ok {
			options.Iterations = int(iterations)
		} else {
			return nil, fmt.Errorf("iterations must be a number (received %T). Example: 100", iterationsValue)
		}
	}

	// Parse stages
	if stagesValue, exists := args["stages"]; exists {
		stagesData, err := json.Marshal(stagesValue)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal stages: %w. Expected array of {duration, target} objects", err)
		}

		var stages []runner.Stage
		if err := json.Unmarshal(stagesData, &stages); err != nil {
			return nil, fmt.Errorf("invalid stages format: %w. Example: [{\"duration\": \"30s\", \"target\": 10}, {\"duration\": \"1m\", \"target\": 20}]", err)
		}
		options.Stages = stages
	}

	// Parse additional options
	if optionsValue, exists := args["options"]; exists {
		if opts, ok := optionsValue.(map[string]interface{}); ok {
			options.Options = opts
		} else {
			return nil, fmt.Errorf("options must be an object (received %T). Example: {\"thresholds\": {\"http_req_duration\": [\"p(95)<500\"]}}", optionsValue)
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
		return mcp.NewToolResultError("Missing required parameter 'query'. Search for development workflow help: 'script validation', 'threshold setup', 'HTTP patterns'. For troubleshooting: 'debugging errors', 'common issues'. For learning: 'getting started', 'examples', 'best practices'."), nil
	}

	query, ok := queryValue.(string)
	if !ok {
		err := fmt.Errorf("query parameter must be a string")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError("Parameter 'query' must be a string containing your search terms. Received: " + fmt.Sprintf("%T", queryValue)), nil
	}

	if query == "" {
		err := fmt.Errorf("query parameter cannot be empty")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError("Query parameter cannot be empty. For script development: 'validation errors', 'threshold patterns', 'HTTP authentication'. For troubleshooting: 'failed checks', 'timeout issues'. For learning: 'getting started', 'examples', 'best practices'."), nil
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
			return mcp.NewToolResultError("Parameter 'max_results' must be a number between 1 and 20. Received: " + fmt.Sprintf("%T", maxResultsValue)), nil
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

	// Enhance query with context-aware terms
	enhancedQuery := enhanceSearchQuery(query)

	results, err := client.Search(ctx, enhancedQuery)
	if err != nil {
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	// Log search results
	logging.SearchEvent(ctx, enhancedQuery, len(results), time.Since(startTime), nil)

	// Generate proactive suggestions
	suggestions := generateSearchSuggestions(query, results)
	relatedQueries := generateRelatedQueries(query)

	// Create enhanced response structure
	response := struct {
		Query          string          `json:"query"`
		EnhancedQuery  string          `json:"enhanced_query,omitempty"`
		Results        []search.Result `json:"results"`
		ResultCount    int             `json:"result_count"`
		Suggestions    []string        `json:"suggestions,omitempty"`
		RelatedQueries []string        `json:"related_queries,omitempty"`
		SearchTips     []string        `json:"search_tips,omitempty"`
		NextSteps      []string        `json:"next_steps,omitempty"`
	}{
		Query:          query,
		EnhancedQuery:  enhancedQuery,
		Results:        results,
		ResultCount:    len(results),
		Suggestions:    suggestions,
		RelatedQueries: relatedQueries,
		SearchTips:     generateSearchTips(query, len(results)),
		NextSteps:      generateSearchNextSteps(query, len(results)),
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

// enhanceSearchQuery adds context-aware terms to improve search relevance
func enhanceSearchQuery(query string) string {
	queryLower := strings.ToLower(query)

	// Add k6-specific context to generic terms
	enhancements := map[string]string{
		"http":            "http requests k6",
		"api":             "api testing k6",
		"load":            "load testing k6",
		"performance":     "performance testing k6",
		"test":            "testing k6",
		"stress":          "stress testing k6",
		"metrics":         "k6 metrics",
		"checks":          "k6 checks",
		"thresholds":      "k6 thresholds",
		"stages":          "k6 stages ramping",
		"vu":              "virtual users k6",
		"virtual users":   "k6 virtual users",
		"javascript":      "k6 javascript",
		"browser":         "k6 browser testing",
		"websocket":       "k6 websocket",
		"graphql":         "k6 graphql",
		"scenarios":       "k6 scenarios",
		"options":         "k6 options configuration",
		"output":          "k6 output results",
		"installation":    "k6 installation setup",
		"getting started": "k6 getting started tutorial",
	}

	// Check if query matches any enhancement patterns
	for pattern, enhancement := range enhancements {
		if strings.Contains(queryLower, pattern) {
			return enhancement
		}
	}

	// If no specific enhancement, add general k6 context
	if !strings.Contains(queryLower, "k6") {
		return query + " k6"
	}

	return query
}

// generateSearchSuggestions provides helpful suggestions based on search results
func generateSearchSuggestions(query string, results []search.Result) []string {
	var suggestions []string
	queryLower := strings.ToLower(query)

	// Content-based suggestions
	if len(results) == 0 {
		suggestions = append(suggestions,
			"Try broader terms like 'getting started' or 'examples'",
			"Check spelling or try synonyms",
			"Use specific k6 feature names like 'http', 'checks', or 'thresholds'",
		)
	} else if len(results) < 3 {
		suggestions = append(suggestions,
			"Try related terms to find more relevant documentation",
			"Consider using more general terms for broader coverage",
		)
	}

	// Query-specific suggestions
	if strings.Contains(queryLower, "error") || strings.Contains(queryLower, "fail") {
		suggestions = append(suggestions, "Also search for 'debugging' and 'troubleshooting' guides")
	}

	if strings.Contains(queryLower, "install") {
		suggestions = append(suggestions, "Look for 'getting started' guides after installation")
	}

	if strings.Contains(queryLower, "example") {
		suggestions = append(suggestions, "Try searching for specific use cases like 'API testing' or 'load testing'")
	}

	return suggestions
}

// generateRelatedQueries suggests related search queries
func generateRelatedQueries(query string) []string {
	queryLower := strings.ToLower(query)

	// Define query relationships
	relatedMap := map[string][]string{
		"http":            {"api testing", "REST API", "HTTP methods", "requests"},
		"api":             {"HTTP requests", "REST API", "GraphQL", "API testing"},
		"load":            {"stress testing", "performance", "virtual users", "stages"},
		"performance":     {"load testing", "metrics", "thresholds", "optimization"},
		"metrics":         {"checks", "thresholds", "output", "monitoring"},
		"checks":          {"assertions", "validation", "metrics", "thresholds"},
		"thresholds":      {"checks", "SLA", "performance criteria", "metrics"},
		"stages":          {"ramping", "load profiles", "scenarios", "virtual users"},
		"virtual users":   {"VU", "concurrency", "load testing", "stages"},
		"browser":         {"browser testing", "UI testing", "web testing"},
		"websocket":       {"real-time", "WebSocket testing", "streaming"},
		"graphql":         {"GraphQL testing", "API testing", "queries"},
		"installation":    {"setup", "getting started", "configuration"},
		"getting started": {"tutorial", "examples", "first test", "basics"},
		"examples":        {"use cases", "tutorials", "patterns", "templates"},
		"debugging":       {"troubleshooting", "errors", "logging", "issues"},
		"scenarios":       {"test organization", "options", "execution", "workflow"},
		"output":          {"results", "reporting", "formats", "analysis"},
	}

	// Find related queries
	var related []string
	for term, queries := range relatedMap {
		if strings.Contains(queryLower, term) {
			related = append(related, queries...)
			break
		}
	}

	// Add general suggestions if no specific matches
	if len(related) == 0 {
		related = []string{
			"getting started",
			"examples",
			"best practices",
			"troubleshooting",
		}
	}

	// Remove duplicates and limit results
	seen := make(map[string]bool)
	var unique []string
	for _, item := range related {
		if !seen[item] && len(unique) < 5 {
			seen[item] = true
			unique = append(unique, item)
		}
	}

	return unique
}

// generateSearchTips provides search optimization tips
func generateSearchTips(query string, resultCount int) []string {
	var tips []string
	queryLower := strings.ToLower(query)

	if resultCount == 0 {
		tips = append(tips,
			"Use specific k6 feature names for better results",
			"Try searching for broader concepts first, then narrow down",
			"Check the official k6 documentation structure with 'getting started'",
		)
	} else if resultCount > 15 {
		tips = append(tips,
			"Use more specific terms to narrow your results",
			"Combine multiple keywords for precise matches",
		)
	}

	if len(query) < 5 {
		tips = append(tips, "Try using longer, more descriptive search terms")
	}

	if !strings.Contains(queryLower, "k6") {
		tips = append(tips, "Include 'k6' in your search for more relevant results")
	}

	// General tips
	if len(tips) == 0 {
		tips = append(tips,
			"Use specific feature names like 'http', 'checks', 'thresholds' for targeted results",
			"Search for 'examples' to find practical use cases",
		)
	}

	return tips
}

// generateSearchNextSteps provides actionable next steps after search
func generateSearchNextSteps(query string, resultCount int) []string {
	var steps []string
	queryLower := strings.ToLower(query)

	if resultCount > 0 {
		steps = append(steps, "Review the search results for relevant documentation")

		if strings.Contains(queryLower, "example") || strings.Contains(queryLower, "tutorial") {
			steps = append(steps, "Try implementing the examples in your own k6 scripts")
		}

		if strings.Contains(queryLower, "install") || strings.Contains(queryLower, "setup") {
			steps = append(steps, "Follow the installation steps, then search for 'getting started'")
		}

		steps = append(steps,
			"Use the 'validate' tool to test your k6 scripts",
			"Search for related topics using the suggested queries",
		)
	} else {
		steps = append(steps,
			"Try the suggested search tips to find relevant documentation",
			"Search for 'getting started' if you're new to k6",
			"Use broader terms first, then search for specific features",
		)
	}

	// Always add general next steps
	steps = append(steps, "Consider bookmarking useful documentation for quick reference")

	return steps
}

// applySmartDefaults applies context-aware defaults to run options
func applySmartDefaults(options *runner.RunOptions, args map[string]interface{}) {
	// Check if user specified any custom parameters
	hasCustomVUs := args["vus"] != nil
	hasCustomDuration := args["duration"] != nil
	hasCustomIterations := args["iterations"] != nil
	hasCustomStages := args["stages"] != nil

	// If no parameters specified, apply intelligent defaults
	if !hasCustomVUs && !hasCustomDuration && !hasCustomIterations && !hasCustomStages {
		// Default to a basic load test
		options.VUs = 1
		options.Duration = "30s"
		return
	}

	// Smart defaults based on what user specified
	if hasCustomStages {
		// If stages are specified, don't set VUs or duration as they'll be controlled by stages
		return
	}

	if hasCustomIterations && !hasCustomVUs {
		// If iterations specified but no VUs, suggest appropriate VU count
		if iterations, ok := args["iterations"].(float64); ok {
			iterationCount := int(iterations)
			switch {
			case iterationCount <= 10:
				options.VUs = 1
			case iterationCount <= 100:
				options.VUs = 5
			case iterationCount <= 1000:
				options.VUs = 10
			default:
				options.VUs = 20
			}
		}
	}

	if hasCustomVUs && !hasCustomDuration && !hasCustomIterations {
		// If VUs specified but no duration/iterations, suggest appropriate duration
		if vus, ok := args["vus"].(float64); ok {
			vusCount := int(vus)
			switch {
			case vusCount <= 5:
				options.Duration = "1m"
			case vusCount <= 20:
				options.Duration = "2m"
			default:
				options.Duration = "5m"
			}
		}
	}
}

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

// suggestParameterImprovements suggests better parameter combinations
func suggestParameterImprovements(args map[string]interface{}) []string {
	var suggestions []string

	// Check for common parameter issues
	if vus, hasVUs := args["vus"].(float64); hasVUs {
		if duration, hasDuration := args["duration"].(string); hasDuration {
			vusCount := int(vus)

			// Suggest improvements for VU/duration combinations
			if vusCount == 1 && duration == "30s" {
				suggestions = append(suggestions,
					"Consider increasing VUs to 5-10 for more realistic load testing",
					"Try extending duration to 2-5 minutes for better insights",
				)
			}

			if vusCount > 20 && duration == "30s" {
				suggestions = append(suggestions,
					"With high VU count, consider longer duration (5-10 minutes) for stability",
				)
			}
		}

		if iterations, hasIterations := args["iterations"].(float64); hasIterations {
			iterationCount := int(iterations)
			vusCount := int(vus)

			// Check for inefficient VU/iteration combinations
			if iterationCount < vusCount {
				suggestions = append(suggestions,
					fmt.Sprintf("You have more VUs (%d) than iterations (%d). Consider increasing iterations or reducing VUs", vusCount, iterationCount),
				)
			}

			if iterationCount > vusCount*100 {
				suggestions = append(suggestions,
					"Very high iteration count per VU. Consider using duration-based testing instead",
				)
			}
		}
	}

	// Check for missing important parameters
	if _, hasVUs := args["vus"]; !hasVUs {
		if _, hasStages := args["stages"]; !hasStages {
			suggestions = append(suggestions, "Consider specifying VUs for better load simulation")
		}
	}

	// Suggest preset configurations
	suggestions = append(suggestions, "Try preset configurations: 'smoke_test', 'load_test', 'stress_test' for common scenarios")

	return suggestions
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
