package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/oleiade/k6-mcp/internal/logging"
	"github.com/oleiade/k6-mcp/internal/validator"
	"log/slog"
	"time"
)

type ValidationHandler struct{}

func NewValidationHandler() *ValidationHandler {
	return &ValidationHandler{}
}

func (v ValidationHandler) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
