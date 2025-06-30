// Package search provides k6 documentation search functionality using embeddings.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbeddingResponse represents the response from the embedding API.
type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Result represents a search result from the documentation.
type Result struct {
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Options configures search behavior.
type Options struct {
	MaxResults     int    `json:"max_results"`
	EmbeddingURL   string `json:"embedding_url"`
	ChromaURL      string `json:"chroma_url"`
	CollectionName string `json:"collection_name"`
}

const (
	defaultMaxResults = 5
	defaultTimeout    = 30
)

// DefaultOptions returns default search configuration.
func DefaultOptions() *Options {
	return &Options{
		MaxResults:     defaultMaxResults,
		EmbeddingURL:   "http://localhost:5001/embed",
		ChromaURL:      "http://localhost:8000",
		CollectionName: "k6_docs",
	}
}

// Client provides search functionality for k6 documentation.
type Client struct {
	httpClient *http.Client
	options    *Options
}

// NewClient creates a new search client with default options.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout * time.Second,
		},
		options: DefaultOptions(),
	}
}

// NewClientWithOptions creates a new search client with custom options.
func NewClientWithOptions(options *Options) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout * time.Second,
		},
		options: options,
	}
}

// getQueryEmbedding calls the embedding API to get an embedding for the search query.
func (c *Client) getQueryEmbedding(queryText string) ([]float32, error) {
	requestBody, err := json.Marshal(map[string]string{"text": queryText})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	resp, err := c.httpClient.Post(c.options.EmbeddingURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call embedding API: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't override the original error
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var embeddingResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	return embeddingResp.Embedding, nil
}

// Search searches the k6 documentation for content similar to the query.
func (c *Client) Search(_ context.Context, query string) ([]Result, error) {
	// For now, return a placeholder implementation that indicates the search functionality
	// is not yet fully implemented. The user will need to:
	// 1. Set up ChromaDB collection with embedded k6 docs
	// 2. Ensure the embedding API is running on port 5001
	// 3. Configure the proper ChromaDB connection
	
	return []Result{
		{
			Content: fmt.Sprintf("Search functionality is being implemented. Query: '%s'", query),
			Metadata: map[string]string{
				"status": "placeholder",
				"note":   "ChromaDB integration requires proper setup of collection and embeddings",
			},
		},
	}, nil
}