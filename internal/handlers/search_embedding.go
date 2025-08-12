package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/oleiade/k6-mcp/internal/logging"
	search2 "github.com/oleiade/k6-mcp/internal/search"
	"strings"
	"time"
)

type EmbeddingSearch struct{}

const (
	maxResultsLimit = 10
)

func (e EmbeddingSearch) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// handleSearch handles the search tool requests.
	// Add request correlation ID
	requestID := uuid.New().String()
	ctx = logging.ContextWithRequestID(ctx, requestID)
	startTime := time.Now()

	args := request.GetArguments()

	// Log request start
	logging.RequestStart(ctx, "search", args)

	// Extract query from arguments
	queryValue, exists := args["keywords"]
	if !exists {
		err := fmt.Errorf("missing required parameter: keywords")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError("Missing required parameter 'keywords'. Search for development workflow help: '\"script validation\"', '\"threshold setup\"', '\"HTTP patterns\"'. For troubleshooting: '\"debugging errors\"', '\"common issues\"'. For learning: '\"getting started\"', '\"examples\"', '\"best practices\"'."), nil
	}

	query, ok := queryValue.(string)
	if !ok {
		err := fmt.Errorf("keywords parameter must be a string")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError("Parameter 'keywords' must be a string containing your search terms. Multi-word queries should be quoted. Received: " + fmt.Sprintf("%T", queryValue)), nil
	}

	if query == "" {
		err := fmt.Errorf("keywords parameter cannot be empty")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError("Keywords parameter cannot be empty. Search for development workflow help: '\"script validation\"', '\"threshold setup\"', '\"HTTP patterns\"'. For troubleshooting: '\"debugging errors\"', '\"common issues\"'. For learning: '\"getting started\"', '\"examples\"', '\"best practices\"'."), nil
	}

	// Parse search options
	options := search2.DefaultOptions()

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
	client := search2.NewEmbeddingSearch()
	//client, err := search.NewSearch(search.BackendChroma, options)
	//if err != nil {
	//	logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
	//	return mcp.NewToolResultError(fmt.Sprintf("failed to create search client: %v", err)), nil
	//}
	//defer func() {
	//	if closeErr := client.Close(); closeErr != nil {
	//		logging.WithContext(ctx).Warn("Failed to close search client",
	//			slog.String("error", closeErr.Error()),
	//		)
	//	}
	//}()

	// Enhance query with context-aware terms
	enhancedQuery := enhanceSearchQuery(query)

	results, err := client.Search(ctx, enhancedQuery, options)
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
		Query          string           `json:"query"`
		EnhancedQuery  string           `json:"enhanced_query,omitempty"`
		Results        []search2.Result `json:"results"`
		ResultCount    int              `json:"result_count"`
		Suggestions    []string         `json:"suggestions,omitempty"`
		RelatedQueries []string         `json:"related_queries,omitempty"`
		SearchTips     []string         `json:"search_tips,omitempty"`
		NextSteps      []string         `json:"next_steps,omitempty"`
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
func generateSearchSuggestions(query string, results []search2.Result) []string {
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
