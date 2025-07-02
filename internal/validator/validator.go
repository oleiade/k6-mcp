// Package validator provides k6 script validation functionality.
package validator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/oleiade/k6-mcp/internal/logging"
)

const (
	// DefaultTimeout is the default timeout for k6 validation runs.
	DefaultTimeout = 30 * time.Second
	// MaxScriptSize is the maximum allowed script size in bytes (1MB).
	MaxScriptSize = 1024 * 1024
)

// ValidationResult contains the result of a k6 script validation.
type ValidationResult struct {
	Valid     bool   `json:"valid"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Error     string `json:"error,omitempty"`
	Duration  string `json:"duration"`
	ScriptURL string `json:"script_url,omitempty"`
}

// ValidationError represents errors that occur during validation.
type ValidationError struct {
	Type    string
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// ValidateK6Script validates a k6 script by executing it with minimal configuration.
func ValidateK6Script(ctx context.Context, script string) (*ValidationResult, error) {
	startTime := time.Now()
	logger := logging.WithComponent("validator")

	logger.DebugContext(ctx, "Starting script validation",
		slog.Int("script_size", len(script)),
	)

	// Input validation
	if err := validateInput(script); err != nil {
		logging.ValidationEvent(ctx, "input_validation", false, map[string]interface{}{
			"error": err.Error(),
			"script_size": len(script),
		})
		return &ValidationResult{
			Valid:    false,
			Error:    err.Error(),
			Duration: time.Since(startTime).String(),
		}, err
	}

	logging.ValidationEvent(ctx, "input_validation", true, map[string]interface{}{
		"script_size": len(script),
	})

	// Create secure temporary file
	tempFile, cleanup, err := createSecureTempFile(script)
	if err != nil {
		logging.FileOperation(ctx, "validator", "create_temp_file", tempFile, err)
		return &ValidationResult{
			Valid:    false,
			Error:    fmt.Sprintf("failed to create temporary file: %v", err),
			Duration: time.Since(startTime).String(),
		}, err
	}
	defer cleanup()

	logging.FileOperation(ctx, "validator", "create_temp_file", tempFile, nil)

	// Execute k6 validation
	result, err := executeK6Validation(ctx, tempFile)
	result.Duration = time.Since(startTime).String()

	logger.DebugContext(ctx, "Validation completed",
		slog.Bool("valid", result.Valid),
		slog.Int("exit_code", result.ExitCode),
		slog.Duration("duration", time.Since(startTime)),
	)

	return result, err
}

// validateInput performs basic input validation on the script content.
func validateInput(script string) error {
	if len(script) == 0 {
		return &ValidationError{
			Type:    "INPUT_VALIDATION",
			Message: "script content cannot be empty",
		}
	}

	if len(script) > MaxScriptSize {
		return &ValidationError{
			Type:    "INPUT_VALIDATION",
			Message: fmt.Sprintf("script size exceeds maximum allowed size of %d bytes", MaxScriptSize),
		}
	}

	// Check for potentially dangerous patterns
	dangerousPatterns := []string{
		"require('child_process')",
		"require(\"child_process\")",
		"exec(",
		"spawn(",
		"fork(",
		"execSync(",
		"execFile(",
	}

	scriptLower := strings.ToLower(script)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(scriptLower, strings.ToLower(pattern)) {
			return &ValidationError{
				Type:    "SECURITY_VALIDATION",
				Message: fmt.Sprintf("script contains potentially dangerous pattern: %s", pattern),
			}
		}
	}

	return nil
}

// createSecureTempFile creates a secure temporary file with the script content.
func createSecureTempFile(script string) (string, func(), error) {
	tmpFile, err := os.CreateTemp("", "k6-script-*.js")
	if err != nil {
		return "", nil, &ValidationError{
			Type:    "FILE_CREATION",
			Message: "failed to create temporary file",
			Cause:   err,
		}
	}

	filename := tmpFile.Name()
	cleanup := func() {
		if removeErr := os.Remove(filename); removeErr != nil {
			logging.WithComponent("validator").Warn("Failed to remove temporary file",
				slog.String("operation", "cleanup"),
				slog.String("error", removeErr.Error()),
			)
		}
	}

	if err := setupTempFile(tmpFile, script); err != nil {
		cleanupTempFile(tmpFile)
		return "", nil, err
	}

	return filename, cleanup, nil
}

// setupTempFile configures and writes to the temporary file.
func setupTempFile(tmpFile *os.File, script string) error {
	// Set secure permissions (owner read/write only)
	const secureFileMode = 0o600
	if err := tmpFile.Chmod(secureFileMode); err != nil {
		return &ValidationError{
			Type:    "FILE_PERMISSION",
			Message: "failed to set secure file permissions",
			Cause:   err,
		}
	}

	// Write script content
	if _, err := tmpFile.WriteString(script); err != nil {
		return &ValidationError{
			Type:    "FILE_WRITE",
			Message: "failed to write script to temporary file",
			Cause:   err,
		}
	}

	if err := tmpFile.Close(); err != nil {
		return &ValidationError{
			Type:    "FILE_CLOSE",
			Message: "failed to close temporary file",
			Cause:   err,
		}
	}

	return nil
}

// cleanupTempFile safely cleans up a temporary file.
func cleanupTempFile(tmpFile *os.File) {
	logger := logging.WithComponent("validator")
	
	if closeErr := tmpFile.Close(); closeErr != nil {
		logger.Warn("Failed to close temp file",
			slog.String("operation", "cleanup"),
			slog.String("error", closeErr.Error()),
		)
	}
	if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
		logger.Warn("Failed to remove temp file",
			slog.String("operation", "cleanup"),
			slog.String("error", removeErr.Error()),
		)
	}
}

// executeK6Validation executes k6 with the given script file.
func executeK6Validation(ctx context.Context, scriptPath string) (*ValidationResult, error) {
	logger := logging.WithComponent("validator")
	startTime := time.Now()
	
	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	// Check if k6 is available
	if _, err := exec.LookPath("k6"); err != nil {
		logger.ErrorContext(ctx, "k6 executable not found",
			slog.String("error", err.Error()),
		)
		return &ValidationResult{
			Valid: false,
			Error: "k6 executable not found in PATH",
		}, &ValidationError{
			Type:    "K6_NOT_FOUND",
			Message: "k6 executable not found in PATH",
			Cause:   err,
		}
	}

	// Prepare k6 command with minimal configuration
	cmd := exec.CommandContext(cmdCtx, "k6", "run", "--vus", "1", "--iterations", "1", "--quiet", scriptPath)

	// Set minimal environment
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}

	logger.DebugContext(ctx, "Executing k6 validation command",
		slog.String("command", "k6 run"),
		slog.String("script_path", getPathType(scriptPath)),
	)

	// Execute command and capture output
	stdout, stderr, exitCode, err := executeCommand(cmd)
	
	// Log execution results
	logging.ExecutionEvent(ctx, "validator", "k6 run", time.Since(startTime), exitCode, err)

	result := &ValidationResult{
		Valid:    exitCode == 0,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}

	// Handle different types of errors
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			// Command timed out
			result.Error = fmt.Sprintf("k6 validation timed out after %v", DefaultTimeout)
			return result, &ValidationError{
				Type:    "TIMEOUT",
				Message: fmt.Sprintf("k6 validation timed out after %v", DefaultTimeout),
				Cause:   err,
			}
		default:
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				// Command executed but returned non-zero exit code
				result.Error = fmt.Sprintf("k6 validation failed with exit code %d", exitCode)
			} else {
				// Other execution errors
				result.Error = fmt.Sprintf("failed to execute k6: %v", err)
				return result, &ValidationError{
					Type:    "EXECUTION_ERROR",
					Message: "failed to execute k6 command",
					Cause:   err,
				}
			}
		}
	}

	return result, nil
}

// executeCommand executes a command and returns stdout, stderr, exit code, and error.
func executeCommand(cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error) {
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	if err != nil {
		return stdout, stderr, exitCode, fmt.Errorf("command execution failed: %w", err)
	}
	return stdout, stderr, exitCode, nil
}

// getPathType returns a safe representation of file paths for logging
func getPathType(path string) string {
	if strings.Contains(path, "temp") || strings.Contains(path, "tmp") {
		return "temporary"
	} else if strings.HasSuffix(path, ".js") {
		return "javascript"
	} else if strings.HasSuffix(path, ".ts") {
		return "typescript"
	}
	return "other"
}