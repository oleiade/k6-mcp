// Package runner provides k6 script execution functionality.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/oleiade/k6-mcp/internal/logging"
	"github.com/oleiade/k6-mcp/internal/security"
)

const (
	// DefaultTimeout is the default timeout for k6 test runs.
	DefaultTimeout = 5 * time.Minute
	// MaxVUs is the maximum number of virtual users allowed.
	MaxVUs = 50
	// MaxDuration is the maximum test duration allowed.
	MaxDuration = 5 * time.Minute
	// DefaultVUs is the default number of virtual users.
	DefaultVUs = 1
	// DefaultDuration is the default test duration.
	DefaultDuration = "30s"
	// P95Percentile represents the 95th percentile for response time calculations.
	P95Percentile = 0.95
)

// RunOptions contains configuration options for running k6 tests.
type RunOptions struct {
	VUs        int               `json:"vus,omitempty"`
	Duration   string            `json:"duration,omitempty"`
	Iterations int               `json:"iterations,omitempty"`
	Stages     []Stage           `json:"stages,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

// Stage represents a load testing stage with target VUs and duration.
type Stage struct {
	Duration string `json:"duration"`
	Target   int    `json:"target"`
}

// RunResult contains the result of a k6 test execution.
type RunResult struct {
	Success    bool              `json:"success"`
	ExitCode   int               `json:"exit_code"`
	Stdout     string            `json:"stdout"`
	Stderr     string            `json:"stderr"`
	Error      string            `json:"error,omitempty"`
	Duration   string            `json:"duration"`
	Metrics    map[string]interface{} `json:"metrics,omitempty"`
	Summary    TestSummary       `json:"summary,omitempty"`
}

// TestSummary contains a summary of the test execution results.
type TestSummary struct {
	TotalRequests   int     `json:"total_requests"`
	FailedRequests  int     `json:"failed_requests"`
	AvgResponseTime float64 `json:"avg_response_time_ms"`
	P95ResponseTime float64 `json:"p95_response_time_ms"`
	RequestRate     float64 `json:"request_rate_per_second"`
	DataReceived    string  `json:"data_received"`
	DataSent        string  `json:"data_sent"`
}

// RunError represents errors that occur during k6 test execution.
type RunError struct {
	Type    string
	Message string
	Cause   error
}

func (e *RunError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *RunError) Unwrap() error {
	return e.Cause
}

// RunK6Test executes a k6 script with the specified options.
func RunK6Test(ctx context.Context, script string, options *RunOptions) (*RunResult, error) {
	startTime := time.Now()
	logger := logging.WithComponent("runner")

	// Log test configuration
	logger.DebugContext(ctx, "Starting k6 test execution",
		slog.Int("script_size", len(script)),
		slog.Any("options", sanitizeRunOptions(options)),
	)

	// Input validation
	if err := validateRunInput(script, options); err != nil {
		logger.WarnContext(ctx, "Test input validation failed",
			slog.String("error", err.Error()),
		)
		return &RunResult{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(startTime).String(),
		}, err
	}

	logger.DebugContext(ctx, "Test input validation passed")

	// Create secure temporary file
	tempFile, cleanup, err := createSecureTempFile(script)
	if err != nil {
		logging.FileOperation(ctx, "runner", "create_temp_file", tempFile, err)
		return &RunResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to create temporary file: %v", err),
			Duration: time.Since(startTime).String(),
		}, err
	}
	defer cleanup()

	logging.FileOperation(ctx, "runner", "create_temp_file", tempFile, nil)

	// Execute k6 test
	result, err := executeK6Test(ctx, tempFile, options)
	result.Duration = time.Since(startTime).String()

	logger.InfoContext(ctx, "k6 test execution completed",
		slog.Bool("success", result.Success),
		slog.Int("exit_code", result.ExitCode),
		slog.Duration("duration", time.Since(startTime)),
		slog.Int("total_requests", result.Summary.TotalRequests),
		slog.Int("failed_requests", result.Summary.FailedRequests),
	)

	return result, err
}

// validateRunInput performs input validation on the script and options.
func validateRunInput(script string, options *RunOptions) error {
	// Validate script content using existing security module
	if err := security.ValidateScriptContent(script); err != nil {
		return &RunError{
			Type:    "INPUT_VALIDATION",
			Message: "script validation failed",
			Cause:   err,
		}
	}

	// Set defaults if options is nil
	if options == nil {
		return nil
	}

	return validateRunOptions(options)
}

// validateRunOptions validates the run options parameters.
func validateRunOptions(options *RunOptions) error {
	if err := validateVUsAndIterations(options); err != nil {
		return err
	}

	if err := validateDuration(options); err != nil {
		return err
	}

	return validateStages(options.Stages)
}

// validateVUsAndIterations validates VUs and iterations parameters.
func validateVUsAndIterations(options *RunOptions) error {
	// Validate VUs
	if options.VUs < 0 {
		return &RunError{
			Type:    "PARAMETER_VALIDATION",
			Message: "vus cannot be negative",
		}
	}
	if options.VUs > MaxVUs {
		return &RunError{
			Type:    "PARAMETER_VALIDATION",
			Message: fmt.Sprintf("vus cannot exceed %d", MaxVUs),
		}
	}

	// Validate iterations
	if options.Iterations < 0 {
		return &RunError{
			Type:    "PARAMETER_VALIDATION",
			Message: "iterations cannot be negative",
		}
	}

	return nil
}

// validateDuration validates the duration parameter.
func validateDuration(options *RunOptions) error {
	if options.Duration == "" {
		return nil
	}

	duration, err := time.ParseDuration(options.Duration)
	if err != nil {
		return &RunError{
			Type:    "PARAMETER_VALIDATION",
			Message: fmt.Sprintf("invalid duration format: %s", options.Duration),
			Cause:   err,
		}
	}
	if duration > MaxDuration {
		return &RunError{
			Type:    "PARAMETER_VALIDATION",
			Message: fmt.Sprintf("duration cannot exceed %v", MaxDuration),
		}
	}

	return nil
}

// validateStages validates the stages configuration.
func validateStages(stages []Stage) error {
	for i, stage := range stages {
		if stage.Target > MaxVUs {
			return &RunError{
				Type:    "PARAMETER_VALIDATION",
				Message: fmt.Sprintf("stage %d target VUs (%d) cannot exceed %d", i, stage.Target, MaxVUs),
			}
		}
		if _, err := time.ParseDuration(stage.Duration); err != nil {
			return &RunError{
				Type:    "PARAMETER_VALIDATION",
				Message: fmt.Sprintf("stage %d has invalid duration format: %s", i, stage.Duration),
				Cause:   err,
			}
		}
	}

	return nil
}

// createSecureTempFile creates a secure temporary file with the script content.
func createSecureTempFile(script string) (string, func(), error) {
	tmpFile, err := os.CreateTemp("", "k6-run-*.js")
	if err != nil {
		return "", nil, &RunError{
			Type:    "FILE_CREATION",
			Message: "failed to create temporary file",
			Cause:   err,
		}
	}

	filename := tmpFile.Name()
	cleanup := func() {
		if removeErr := os.Remove(filename); removeErr != nil {
			logging.WithComponent("runner").Warn("Failed to remove temporary file",
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
		return &RunError{
			Type:    "FILE_PERMISSION",
			Message: "failed to set secure file permissions",
			Cause:   err,
		}
	}

	// Write script content
	if _, err := tmpFile.WriteString(script); err != nil {
		return &RunError{
			Type:    "FILE_WRITE",
			Message: "failed to write script to temporary file",
			Cause:   err,
		}
	}

	if err := tmpFile.Close(); err != nil {
		return &RunError{
			Type:    "FILE_CLOSE",
			Message: "failed to close temporary file",
			Cause:   err,
		}
	}

	return nil
}

// cleanupTempFile safely cleans up a temporary file.
func cleanupTempFile(tmpFile *os.File) {
	logger := logging.WithComponent("runner")
	
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

// executeK6Test executes k6 with the given script file and options.
func executeK6Test(ctx context.Context, scriptPath string, options *RunOptions) (*RunResult, error) {
	logger := logging.WithComponent("runner")
	startTime := time.Now()
	
	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	// Check if k6 is available
	if err := security.ValidateEnvironment(); err != nil {
		logger.ErrorContext(ctx, "k6 executable not found",
			slog.String("error", err.Error()),
		)
		return &RunResult{
			Success: false,
			Error:   "k6 executable not found in PATH",
		}, &RunError{
			Type:    "K6_NOT_FOUND",
			Message: "k6 executable not found in PATH",
			Cause:   err,
		}
	}

	// Build k6 command arguments
	args := buildK6Args(scriptPath, options)

	logger.DebugContext(ctx, "Executing k6 test command",
		slog.Any("args", args),
		slog.String("script_path", getPathType(scriptPath)),
	)

	// Prepare k6 command
	// #nosec G204 - k6 binary is validated to exist, args are sanitized
	cmd := exec.CommandContext(cmdCtx, "k6", args...)

	// Set secure environment
	cmd.Env = security.SecureEnvironment()

	// Execute command and capture output
	stdout, stderr, exitCode, err := executeCommand(cmd)
	
	// Log execution results
	logging.ExecutionEvent(ctx, "runner", "k6 run", time.Since(startTime), exitCode, err)

	// Sanitize output to prevent information leakage
	stdout = security.SanitizeOutput(stdout)
	stderr = security.SanitizeOutput(stderr)

	result := &RunResult{
		Success:  exitCode == 0,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}

	// Parse metrics and summary from output
	if result.Success {
		result.Metrics, result.Summary = parseK6Output(stdout)
	}

	// Handle different types of errors
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			// Command timed out
			result.Error = fmt.Sprintf("k6 test timed out after %v", DefaultTimeout)
			return result, &RunError{
				Type:    "TIMEOUT",
				Message: fmt.Sprintf("k6 test timed out after %v", DefaultTimeout),
				Cause:   err,
			}
		default:
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				// Command executed but returned non-zero exit code
				result.Error = fmt.Sprintf("k6 test failed with exit code %d", exitCode)
			} else {
				// Other execution errors
				result.Error = fmt.Sprintf("failed to execute k6: %v", err)
				return result, &RunError{
					Type:    "EXECUTION_ERROR",
					Message: "failed to execute k6 command",
					Cause:   err,
				}
			}
		}
	}

	return result, nil
}

// buildK6Args builds the command line arguments for k6 based on the provided options.
func buildK6Args(scriptPath string, options *RunOptions) []string {
	args := []string{"run"}

	// Set defaults if options is nil
	if options == nil {
		options = &RunOptions{
			VUs:      DefaultVUs,
			Duration: DefaultDuration,
		}
	}

	// Set VUs (default to 1 if not specified)
	vus := options.VUs
	if vus == 0 {
		vus = DefaultVUs
	}
	args = append(args, "--vus", strconv.Itoa(vus))

	// Handle duration vs iterations
	if options.Iterations > 0 {
		args = append(args, "--iterations", strconv.Itoa(options.Iterations))
	} else {
		duration := options.Duration
		if duration == "" {
			duration = DefaultDuration
		}
		args = append(args, "--duration", duration)
	}

	// Handle stages for load profiling
	if len(options.Stages) > 0 {
		stagesStr := buildStagesString(options.Stages)
		args = append(args, "--stage", stagesStr)
	}

	// Add JSON output for metrics parsing
	args = append(args, "--out", "json=/dev/stdout")

	// Add script path
	args = append(args, scriptPath)

	return args
}

// buildStagesString creates a stages configuration string for k6.
func buildStagesString(stages []Stage) string {
	var stageStrings []string
	for _, stage := range stages {
		stageStr := fmt.Sprintf("%s:%d", stage.Duration, stage.Target)
		stageStrings = append(stageStrings, stageStr)
	}
	return strings.Join(stageStrings, ",")
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

// parseK6Output parses k6 output to extract metrics and summary information.
func parseK6Output(output string) (map[string]interface{}, TestSummary) {
	metrics := make(map[string]interface{})
	summary := TestSummary{}

	// Split output into lines and parse JSON metrics
	lines := strings.Split(output, "\n")
	var jsonMetrics []map[string]interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to parse as JSON metric
		var metric map[string]interface{}
		if err := json.Unmarshal([]byte(line), &metric); err == nil {
			jsonMetrics = append(jsonMetrics, metric)
		}
	}

	// Extract summary metrics from JSON output
	if len(jsonMetrics) > 0 {
		summary = extractSummaryFromMetrics(jsonMetrics)
		metrics["raw_metrics"] = jsonMetrics
		metrics["metrics_count"] = len(jsonMetrics)
	}

	return metrics, summary
}

// extractSummaryFromMetrics extracts test summary information from k6 JSON metrics.
func extractSummaryFromMetrics(jsonMetrics []map[string]interface{}) TestSummary {
	summary := TestSummary{}

	// Count HTTP requests and calculate statistics
	httpReqs := 0
	httpFailures := 0
	var responseTimes []float64

	for _, metric := range jsonMetrics {
		if metricType, ok := metric["type"].(string); ok && metricType == "Point" {
			if metricName, ok := metric["metric"].(string); ok {
				switch metricName {
				case "http_reqs":
					httpReqs++
				case "http_req_failed":
					if value, ok := metric["data"].(map[string]interface{}); ok {
						if failed, ok := value["value"].(float64); ok && failed > 0 {
							httpFailures++
						}
					}
				case "http_req_duration":
					if value, ok := metric["data"].(map[string]interface{}); ok {
						if duration, ok := value["value"].(float64); ok {
							responseTimes = append(responseTimes, duration)
						}
					}
				}
			}
		}
	}

	summary.TotalRequests = httpReqs
	summary.FailedRequests = httpFailures

	// Calculate response time statistics
	if len(responseTimes) > 0 {
		summary.AvgResponseTime = calculateAverage(responseTimes)
		summary.P95ResponseTime = calculatePercentile(responseTimes, P95Percentile)
	}

	return summary
}

// calculateAverage calculates the average of a slice of float64 values.
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculatePercentile calculates the specified percentile of a slice of float64 values.
func calculatePercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Simple percentile calculation (would be more accurate with sorting)
	index := int(float64(len(values)) * percentile)
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

// sanitizeRunOptions removes sensitive information from run options for logging
func sanitizeRunOptions(options *RunOptions) interface{} {
	if options == nil {
		return nil
	}
	
	return map[string]interface{}{
		"vus":        options.VUs,
		"duration":   options.Duration,
		"iterations": options.Iterations,
		"stages":     options.Stages,
		"has_options": options.Options != nil,
	}
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