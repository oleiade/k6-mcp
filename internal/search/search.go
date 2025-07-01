// Package search provides k6 documentation search functionality using embeddings.
package search

import (
	"context"
	"fmt"
	"os"

	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	defaultef "github.com/amikos-tech/chroma-go/pkg/embeddings/default_ef"
)

// Result represents a search result from the documentation.
type Result struct {
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Score    float32           `json:"score,omitempty"`
	Source   string            `json:"source,omitempty"`
}

// Options configures search behavior.
type Options struct {
	MaxResults     int    `json:"max_results"`
	ChromaURL      string `json:"chroma_url"`
	CollectionName string `json:"collection_name"`
}

const (
	defaultMaxResults = 5
	defaultTimeout    = 30
)

// DefaultOptions returns default search configuration.
func DefaultOptions() *Options {
	// Default to localhost - works for both local development and Docker with host networking
	chromaURL := "http://localhost:8000"

	// Check for explicit environment variable first to override default
	if envURL := os.Getenv("CHROMA_URL"); envURL != "" {
		chromaURL = envURL
	}

	return &Options{
		MaxResults:     defaultMaxResults,
		ChromaURL:      chromaURL,
		CollectionName: "k6_docs",
	}
}

// isRunningInDocker checks if we're running inside a Docker container
func isRunningInDocker() bool {
	// Simple check for common Docker indicators
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// isRunningInDockerCompose checks if we're running in a Docker Compose network
func isRunningInDockerCompose() bool {
	// Check for Docker Compose specific indicators
	// If we're in Docker and not using host networking, assume Docker Compose
	if !isRunningInDocker() {
		return false
	}

	// Check if we can resolve the chroma hostname (indicates we're in compose network)
	// This is a simple way to detect if we're in the compose network vs host network
	if hostname := os.Getenv("HOSTNAME"); hostname != "" {
		// In compose, containers typically have meaningful hostnames
		return true
	}

	return false
}

// Client provides search functionality for k6 documentation.
type Client struct {
	chromaClient chroma.Client
	collection   chroma.Collection
	options      *Options
}

// NewClient creates a new search client with default options.
func NewClient() *Client {
	return &Client{
		options: DefaultOptions(),
	}
}

// NewClientWithOptions creates a new search client with custom options.
func NewClientWithOptions(options *Options) *Client {
	return &Client{
		options: options,
	}
}

// initializeChromaClient initializes the ChromaDB client and collection if not already done.
func (c *Client) initializeChromaClient(ctx context.Context) error {
	if c.chromaClient != nil && c.collection != nil {
		return nil // Already initialized
	}

	// Create ChromaDB client with custom URL if specified
	var client chroma.Client
	var err error

	if c.options.ChromaURL != "http://localhost:8000" {
		// Custom URL specified
		client, err = chroma.NewHTTPClient(chroma.WithBaseURL(c.options.ChromaURL))
	} else {
		// Use default URL
		client, err = chroma.NewHTTPClient()
	}

	if err != nil {
		return fmt.Errorf("failed to create ChromaDB client: %w", err)
	}

	c.chromaClient = client

	// Create default embedding function (all-MiniLM-L6-v2)
	// This must work - no fallbacks
	ef, _, err := defaultef.NewDefaultEmbeddingFunction()
	if err != nil {
		return fmt.Errorf("failed to initialize default embedding function: %w", err)
	}

	// Get the collection with our embedding function
	collection, err := client.GetCollection(ctx, c.options.CollectionName, chroma.WithEmbeddingFunctionGet(ef))
	if err != nil {
		return fmt.Errorf("failed to get collection '%s': %w", c.options.CollectionName, err)
	}

	c.collection = collection
	return nil
}

// Search searches the k6 documentation for content similar to the query.
func (c *Client) Search(ctx context.Context, query string) ([]Result, error) {
	// Initialize ChromaDB client and collection if needed
	if err := c.initializeChromaClient(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize ChromaDB client: %w", err)
	}

	// Query the collection using the same approach as chroma-test/main.go
	queryResult, err := c.collection.Query(ctx,
		chroma.WithQueryTexts(query),
		chroma.WithNResults(c.options.MaxResults),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query ChromaDB collection: %w", err)
	}

	// Convert ChromaDB query result to our Result format
	results := make([]Result, 0)

	// Get the document groups from the query result
	documentGroups := queryResult.GetDocumentsGroups()
	if len(documentGroups) == 0 {
		return results, nil
	}

	// Process the first group (since we only have one query)
	documents := documentGroups[0]
	metadatas := queryResult.GetMetadatasGroups()
	distances := queryResult.GetDistancesGroups()

	for i, document := range documents {
		// Convert document to string
		docStr := fmt.Sprintf("%v", document)

		result := Result{
			Content: docStr,
		}

		// Add metadata if available
		if len(metadatas) > 0 && i < len(metadatas[0]) && metadatas[0][i] != nil {
			result.Metadata = make(map[string]string)
			metadata := metadatas[0][i]
			// Convert metadata to map[string]string
			metadataMap := fmt.Sprintf("%v", metadata)
			// For now, just store the metadata as a single entry
			result.Metadata["metadata"] = metadataMap

			// Try to extract source if metadata contains it in a recognizable format
			if metadataStr := fmt.Sprintf("%v", metadata); metadataStr != "" {
				result.Source = "k6_docs"
			}
		}

		// Add similarity score (ChromaDB returns distances, lower is better)
		if len(distances) > 0 && i < len(distances[0]) {
			// Convert distance to similarity score (1 - normalized distance)
			distance := distances[0][i]
			result.Score = 1.0 - float32(distance)
		}

		results = append(results, result)
	}

	return results, nil
}

// Close closes the ChromaDB client to release resources.
func (c *Client) Close() error {
	if c.chromaClient != nil {
		return c.chromaClient.Close()
	}
	return nil
}
