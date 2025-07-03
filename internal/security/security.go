// Package security provides security utilities for the k6 MCP server.
package security

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/oleiade/k6-mcp/internal/logging"
)

const (
	// MaxExecutionTime is the maximum allowed execution time for any operation.
	MaxExecutionTime = 60 * time.Second
	// MaxScriptSizeBytes is the maximum allowed script size.
	MaxScriptSizeBytes = 1024 * 1024 // 1MB
)

// Error represents a security-related error.
type Error struct {
	Type    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("security error [%s]: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("security error [%s]: %s", e.Type, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// ValidateScriptContent performs security validation on script content.
func ValidateScriptContent(content string) error {
	logger := logging.WithComponent("security")
	
	logger.Debug("Starting script content validation",
		slog.Int("content_size", len(content)),
	)

	if len(content) == 0 {
		err := &Error{
			Type:    "EMPTY_CONTENT",
			Message: "script content cannot be empty. Try a basic k6 script: import http from 'k6/http'; export default function() { http.get('https://httpbin.org/get'); }",
		}
		
		logging.SecurityEvent(context.Background(), "empty_content", "medium", 
			"Script content validation failed: empty content", 
			map[string]interface{}{
				"content_size": len(content),
			})
		
		return err
	}

	if len(content) > MaxScriptSizeBytes {
		// Auto-suggest content optimization
		suggestions := generateContentOptimizationSuggestions(content)
		suggestionText := ""
		if len(suggestions) > 0 {
			suggestionText = " Suggestions: " + strings.Join(suggestions, "; ")
		}
		
		err := &Error{
			Type:    "SIZE_LIMIT_EXCEEDED",
			Message: fmt.Sprintf(
				"script size (%d bytes) exceeds maximum allowed size (%d bytes).%s",
				len(content), MaxScriptSizeBytes, suggestionText,
			),
		}
		
		logging.SecurityEvent(context.Background(), "size_limit_exceeded", "high", 
			"Script content validation failed: size limit exceeded", 
			map[string]interface{}{
				"content_size": len(content),
				"max_size": MaxScriptSizeBytes,
			})
		
		return err
	}

	// Check for dangerous patterns with auto-correction suggestions
	if err := checkDangerousPatternsWithSuggestions(content); err != nil {
		return err
	}

	// Check for common script issues and provide suggestions
	if suggestions := detectScriptIssuesWithSuggestions(content); len(suggestions) > 0 {
		logger.Debug("Script content validation passed with suggestions",
			slog.Int("content_size", len(content)),
			slog.Int("suggestion_count", len(suggestions)),
		)
	} else {
		logger.Debug("Script content validation passed",
			slog.Int("content_size", len(content)),
		)
	}

	return nil
}

// checkDangerousPatterns scans the script content for potentially dangerous patterns.
func checkDangerousPatterns(content string) error {
	// Patterns that could indicate attempts to execute system commands or access forbidden APIs
	dangerousPatterns := map[string]string{
		"require('child_process')":     "child process execution",
		"require(\"child_process\")":   "child process execution",
		"require('fs')":                "file system access",
		"require(\"fs\")":              "file system access",
		"require('os')":                "operating system access",
		"require(\"os\")":              "operating system access",
		"require('process')":           "process manipulation",
		"require(\"process\")":         "process manipulation",
		"exec(":                        "command execution",
		"execSync(":                    "synchronous command execution",
		"spawn(":                       "process spawning",
		"fork(":                        "process forking",
		"execFile(":                    "file execution",
		"eval(":                        "code evaluation",
		"Function(":                    "dynamic function creation",
		"new Function(":                "dynamic function creation",
		"import(":                      "dynamic import",
		"require.resolve(":             "module resolution",
		"process.env":                  "environment variable access",
		"process.argv":                 "command line argument access",
		"process.exit":                 "process termination",
		"process.kill":                 "process killing",
		"__dirname":                    "directory path access",
		"__filename":                   "file path access",
		"global.":                      "global object manipulation",
		"globalThis.":                  "global object manipulation",
		"Buffer.":                      "buffer manipulation",
		"setImmediate(":                "immediate execution",
		"setInterval(":                 "interval execution",
		"setTimeout(":                  "timeout execution",
		"clearImmediate(":              "immediate clearing",
		"clearInterval(":               "interval clearing",
		"clearTimeout(":                "timeout clearing",
	}

	contentLower := strings.ToLower(content)

	for pattern, description := range dangerousPatterns {
		if strings.Contains(contentLower, strings.ToLower(pattern)) {
			err := &Error{
				Type:    "DANGEROUS_PATTERN",
				Message: fmt.Sprintf("script contains potentially dangerous pattern related to %s: %s", description, pattern),
			}
			
			logging.SecurityEvent(context.Background(), "dangerous_pattern_detected", "critical", 
				"Dangerous pattern detected in script content", 
				map[string]interface{}{
					"pattern": pattern,
					"description": description,
					"content_size": len(content),
				})
			
			return err
		}
	}

	return nil
}

// CreateSecureContext creates a context with security constraints.
func CreateSecureContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, MaxExecutionTime)
}

// SanitizeOutput sanitizes output strings to prevent information leakage.
func SanitizeOutput(output string) string {
	// Remove potentially sensitive information from output
	sensitive := []string{
		os.Getenv("HOME"),
		os.Getenv("USER"),
		os.Getenv("USERNAME"),
		os.Getenv("LOGNAME"),
	}

	sanitized := output
	for _, s := range sensitive {
		if s != "" {
			sanitized = strings.ReplaceAll(sanitized, s, "[REDACTED]")
		}
	}

	return sanitized
}

// ValidateEnvironment validates that the required tools are available and properly configured.
func ValidateEnvironment() error {
	logger := logging.WithComponent("security")
	
	logger.Debug("Validating environment dependencies")
	
	// Check if k6 is available in PATH
	if _, err := exec.LookPath("k6"); err != nil {
		securityErr := &Error{
			Type:    "MISSING_DEPENDENCY",
			Message: "k6 executable not found in PATH",
			Cause:   err,
		}
		
		logging.SecurityEvent(context.Background(), "missing_dependency", "high", 
			"Required dependency not found in environment", 
			map[string]interface{}{
				"dependency": "k6",
				"error": err.Error(),
			})
		
		return securityErr
	}

	logger.Debug("Environment validation passed")
	return nil
}

// SecureEnvironment returns a minimal, secure environment for command execution.
func SecureEnvironment() []string {
	logger := logging.WithComponent("security")
	
	// Provide only essential environment variables
	essential := []string{
		"PATH=" + os.Getenv("PATH"),
	}

	// Add HOME only if it exists and is not empty
	if home := os.Getenv("HOME"); home != "" {
		essential = append(essential, "HOME="+home)
	}

	logger.Debug("Created secure environment",
		slog.Int("env_var_count", len(essential)),
		slog.Bool("has_home", os.Getenv("HOME") != ""),
	)

	return essential
}

// generateContentOptimizationSuggestions provides suggestions for reducing script size
func generateContentOptimizationSuggestions(content string) []string {
	var suggestions []string
	
	// Count lines and estimate compression opportunities
	lines := strings.Split(content, "\n")
	
	// Check for common size issues
	if len(lines) > 1000 {
		suggestions = append(suggestions, "Consider splitting your script into multiple modules")
	}
	
	// Check for repeated patterns
	if strings.Count(content, "console.log") > 10 {
		suggestions = append(suggestions, "Remove excessive console.log statements")
	}
	
	// Check for large comments
	commentLines := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			commentLines++
		}
	}
	
	if commentLines > len(lines)/4 {
		suggestions = append(suggestions, "Consider reducing inline comments")
	}
	
	// Check for whitespace
	if len(content) - len(strings.ReplaceAll(content, " ", "")) > len(content)/3 {
		suggestions = append(suggestions, "Minimize unnecessary whitespace")
	}
	
	return suggestions
}

// checkDangerousPatternsWithSuggestions scans for dangerous patterns and provides corrections
func checkDangerousPatternsWithSuggestions(content string) error {
	// Enhanced patterns with specific suggestions
	dangerousPatterns := map[string]PatternInfo{
		"require('child_process')":     {Description: "child process execution", Suggestion: "Use k6's built-in HTTP module for external requests"},
		"require(\"child_process\")":   {Description: "child process execution", Suggestion: "Use k6's built-in HTTP module for external requests"},
		"require('fs')":                {Description: "file system access", Suggestion: "Use k6's data loading features or environment variables"},
		"require(\"fs\")":              {Description: "file system access", Suggestion: "Use k6's data loading features or environment variables"},
		"require('os')":                {Description: "operating system access", Suggestion: "Use k6's environment variable access instead"},
		"require(\"os\")":              {Description: "operating system access", Suggestion: "Use k6's environment variable access instead"},
		"require('process')":           {Description: "process manipulation", Suggestion: "Use k6's VU context and built-in functions"},
		"require(\"process\")":         {Description: "process manipulation", Suggestion: "Use k6's VU context and built-in functions"},
		"exec(":                        {Description: "command execution", Suggestion: "Replace with k6 HTTP requests or built-in functions"},
		"execSync(":                    {Description: "synchronous command execution", Suggestion: "Replace with k6 HTTP requests or built-in functions"},
		"spawn(":                       {Description: "process spawning", Suggestion: "Use k6's HTTP module for external communication"},
		"fork(":                        {Description: "process forking", Suggestion: "Use k6's scenarios for concurrent testing"},
		"execFile(":                    {Description: "file execution", Suggestion: "Replace with k6 built-in functionality"},
		"eval(":                        {Description: "code evaluation", Suggestion: "Avoid dynamic code execution in k6 scripts"},
		"Function(":                    {Description: "dynamic function creation", Suggestion: "Use static function definitions in k6"},
		"new Function(":                {Description: "dynamic function creation", Suggestion: "Use static function definitions in k6"},
		"import(":                      {Description: "dynamic import", Suggestion: "Use static import statements at the top of your script"},
	}

	contentLower := strings.ToLower(content)

	for pattern, info := range dangerousPatterns {
		if strings.Contains(contentLower, strings.ToLower(pattern)) {
			err := &Error{
				Type:    "DANGEROUS_PATTERN",
				Message: fmt.Sprintf("Script contains potentially dangerous pattern related to %s: %s. %s", info.Description, pattern, info.Suggestion),
			}
			
			logging.SecurityEvent(context.Background(), "dangerous_pattern_detected", "critical", 
				"Dangerous pattern detected in script content", 
				map[string]interface{}{
					"pattern": pattern,
					"description": info.Description,
					"suggestion": info.Suggestion,
					"content_size": len(content),
				})
			
			return err
		}
	}

	return nil
}

// detectScriptIssuesWithSuggestions detects common k6 script issues and provides suggestions
func detectScriptIssuesWithSuggestions(content string) []string {
	var suggestions []string
	contentLower := strings.ToLower(content)
	
	// Check for missing imports
	if !strings.Contains(contentLower, "import") && strings.Contains(contentLower, "http.get") {
		suggestions = append(suggestions, "Add missing import: import http from 'k6/http';")
	}
	
	// Check for missing default function
	if !strings.Contains(contentLower, "export default function") {
		suggestions = append(suggestions, "Add default export function: export default function() { /* your test code */ }")
	}
	
	// Check for common typos
	typos := map[string]string{
		"htpp":     "http",
		"chekc":    "check",
		"slepe":    "sleep",
		"functoin": "function",
		"improt":   "import",
		"exprot":   "export",
	}
	
	for typo, correction := range typos {
		if strings.Contains(contentLower, typo) {
			suggestions = append(suggestions, fmt.Sprintf("Possible typo: '%s' should be '%s'", typo, correction))
		}
	}
	
	// Check for inefficient patterns
	if strings.Count(content, "http.get") > 10 && !strings.Contains(content, "batch") {
		suggestions = append(suggestions, "Consider using http.batch() for multiple requests")
	}
	
	// Check for missing error handling
	if strings.Contains(content, "http.") && !strings.Contains(content, "check(") {
		suggestions = append(suggestions, "Consider adding checks to validate response status")
	}
	
	// Check for hardcoded values
	if strings.Contains(content, "http://localhost") || strings.Contains(content, "127.0.0.1") {
		suggestions = append(suggestions, "Consider using environment variables for URLs")
	}
	
	return suggestions
}

// PatternInfo contains information about dangerous patterns
type PatternInfo struct {
	Description string
	Suggestion  string
}