package handlers

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/oleiade/k6-mcp/internal/logging"
	"os"
	"strings"
	"time"
)

//go:embed templates/generate_script.md
var generateScriptTemplate string

type ScriptGenerator struct{}

var _ PromptHandler = &ScriptGenerator{}

func NewScriptGenerator() *ScriptGenerator {
	return &ScriptGenerator{}
}

func (s ScriptGenerator) Handle(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
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

	// Load prompt template from file or embedded content
	templateContent, err := loadPromptTemplate("resources/prompts/generate_script.md", generateScriptTemplate)
	if err != nil {
		logging.RequestEnd(ctx, "generate_script", false, time.Since(startTime), err)
		return nil, fmt.Errorf("failed to load prompt template: %w", err)
	}

	// Replace template variables
	promptText := strings.Replace(templateContent, "{{.Description}}", description, 1)

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

// loadPromptTemplate loads the prompt template with filesystem fallback
func loadPromptTemplate(templatePath string, embeddedTemplate string) (string, error) {
	// Try to load from filesystem first (for development)
	if content, err := os.ReadFile(templatePath); err == nil {
		return string(content), nil
	}

	// Fallback to the embedded template
	if embeddedTemplate != "" {
		return embeddedTemplate, nil
	}

	return "", fmt.Errorf("template not found: %s", templatePath)
}
