// Package security provides security utilities for the k6 MCP server.
package security

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
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
	if len(content) == 0 {
		return &Error{
			Type:    "EMPTY_CONTENT",
			Message: "script content cannot be empty",
		}
	}

	if len(content) > MaxScriptSizeBytes {
		return &Error{
			Type:    "SIZE_LIMIT_EXCEEDED",
			Message: fmt.Sprintf(
				"script size (%d bytes) exceeds maximum allowed size (%d bytes)",
				len(content), MaxScriptSizeBytes,
			),
		}
	}

	// Check for dangerous patterns that could be used for code injection or system access
	if err := checkDangerousPatterns(content); err != nil {
		return err
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
			return &Error{
				Type:    "DANGEROUS_PATTERN",
				Message: fmt.Sprintf("script contains potentially dangerous pattern related to %s: %s", description, pattern),
			}
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
	// Check if k6 is available in PATH
	if _, err := exec.LookPath("k6"); err != nil {
		return &Error{
			Type:    "MISSING_DEPENDENCY",
			Message: "k6 executable not found in PATH",
			Cause:   err,
		}
	}

	return nil
}

// SecureEnvironment returns a minimal, secure environment for command execution.
func SecureEnvironment() []string {
	// Provide only essential environment variables
	essential := []string{
		"PATH=" + os.Getenv("PATH"),
	}

	// Add HOME only if it exists and is not empty
	if home := os.Getenv("HOME"); home != "" {
		essential = append(essential, "HOME="+home)
	}

	return essential
}