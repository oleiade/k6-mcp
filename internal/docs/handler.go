package docs

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Handler manages k6 documentation resources for MCP servers.
type Handler struct {
	scanner *Scanner
	parser  *Parser
}

// NewHandler creates a new documentation handler with the specified documentation path.
func NewHandler(docsPath string) *Handler {
	return &Handler{
		scanner: NewScanner(docsPath),
		parser:  NewParser(),
	}
}

// ListResources returns a list of all available k6 documentation resources.
func (h *Handler) ListResources(_ context.Context, _ mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	docs, err := h.scanner.ScanDocuments()
	if err != nil {
		return nil, fmt.Errorf("failed to scan documents: %w", err)
	}
	
	var resources []mcp.Resource
	
	for _, doc := range docs {
		description := fmt.Sprintf("k6 documentation: %s", doc.Title)
		if doc.Version != "" && doc.Category != "" {
			description = fmt.Sprintf("k6 %s documentation: %s - %s", doc.Version, doc.Category, doc.Title)
		}
		
		mimeType := "text/markdown"
		
		resource := mcp.Resource{
			URI:         doc.URI,
			Name:        doc.Title,
			Description: description,
			MIMEType:    mimeType,
		}
		
		resources = append(resources, resource)
	}
	
	return &mcp.ListResourcesResult{
		Resources: resources,
	}, nil
}

// ReadResource retrieves the content of a specific k6 documentation resource.
func (h *Handler) ReadResource(_ context.Context, request mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	doc, err := h.scanner.GetDocumentByURI(request.Params.URI)
	if err != nil {
		var docsErr *Error
		if errors.As(err, &docsErr) {
			switch docsErr.Type {
			case ErrorTypeNotFound:
				return nil, fmt.Errorf("resource not found: %s", request.Params.URI)
			case ErrorTypeValidation:
				return nil, fmt.Errorf("invalid resource URI: %s", request.Params.URI)
			case ErrorTypeIO:
				return nil, fmt.Errorf("IO error accessing document: %w", err)
			case ErrorTypeInternal:
				return nil, fmt.Errorf("internal error: %w", err)
			default:
				return nil, fmt.Errorf("failed to get document: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to get document: %w", err)
		}
	}
	
	content, err := h.parser.ParseDocument(doc.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}
	
	resourceContent := h.formatResourceContent(doc, content)
	
	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "text/markdown",
				Text:     resourceContent,
			},
		},
	}, nil
}

func (h *Handler) formatResourceContent(doc *DocumentInfo, content *DocumentContent) string {
	var result strings.Builder
	
	result.WriteString(fmt.Sprintf("# %s\n\n", content.Title))
	
	result.WriteString(fmt.Sprintf("**Document URI:** %s\n", doc.URI))
	result.WriteString(fmt.Sprintf("**Version:** %s\n", doc.Version))
	result.WriteString(fmt.Sprintf("**Category:** %s\n", doc.Category))
	if doc.Subcategory != "" {
		result.WriteString(fmt.Sprintf("**Subcategory:** %s\n", doc.Subcategory))
	}
	result.WriteString("\n---\n\n")
	
	if len(content.Frontmatter) > 0 {
		result.WriteString("## Metadata\n\n")
		for key, value := range content.Frontmatter {
			result.WriteString(fmt.Sprintf("- **%s:** %s\n", key, value))
		}
		result.WriteString("\n")
	}
	
	result.WriteString("## Content\n\n")
	result.WriteString(content.Content)
	
	return result.String()
}

