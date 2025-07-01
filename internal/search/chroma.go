package search

import (
	"context"
	"fmt"
	"os"

	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	defaultef "github.com/amikos-tech/chroma-go/pkg/embeddings/default_ef"
)

const (
	// DefaultChromaURL is the default ChromaDB server URL.
	DefaultChromaURL = "http://localhost:8000"
)

// ChromaOptions extends the generic Options with ChromaDB-specific configuration.
type ChromaOptions struct {
	*Options
	URL string `json:"url"`
}

// DefaultChromaOptions returns default ChromaDB search configuration.
func DefaultChromaOptions() *ChromaOptions {
	// Default to localhost - works for both local development and Docker with host networking
	chromaURL := DefaultChromaURL

	// Check for explicit environment variable first to override default
	if envURL := os.Getenv("CHROMA_URL"); envURL != "" {
		chromaURL = envURL
	}

	return &ChromaOptions{
		Options: DefaultOptions(),
		URL:     chromaURL,
	}
}

// ChromaSearch implements the Search interface using ChromaDB.
type ChromaSearch struct {
	chromaClient chroma.Client
	collection   chroma.Collection
	options      *ChromaOptions
}

// NewChromaSearch creates a new ChromaDB search client.
func NewChromaSearch(options *Options) (*ChromaSearch, error) {
	chromaOpts := &ChromaOptions{
		Options: options,
		URL:     DefaultChromaURL,
	}

	// Check for explicit environment variable to override default
	if envURL := os.Getenv("CHROMA_URL"); envURL != "" {
		chromaOpts.URL = envURL
	}

	return &ChromaSearch{
		options: chromaOpts,
	}, nil
}

// NewChromaSearchWithOptions creates a new ChromaDB search client with ChromaDB-specific options.
func NewChromaSearchWithOptions(options *ChromaOptions) (*ChromaSearch, error) {
	if options == nil {
		options = DefaultChromaOptions()
	}
	if options.Options == nil {
		options.Options = DefaultOptions()
	}

	return &ChromaSearch{
		options: options,
	}, nil
}

// initializeChromaClient initializes the ChromaDB client and collection if not already done.
func (c *ChromaSearch) initializeChromaClient(ctx context.Context) error {
	if c.chromaClient != nil && c.collection != nil {
		return nil // Already initialized
	}

	// Create ChromaDB client with custom URL if specified
	var client chroma.Client
	var err error

	if c.options.URL != DefaultChromaURL {
		// Custom URL specified
		client, err = chroma.NewHTTPClient(chroma.WithBaseURL(c.options.URL))
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
func (c *ChromaSearch) Search(ctx context.Context, query string) ([]Result, error) {
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
func (c *ChromaSearch) Close() error {
	if c.chromaClient != nil {
		if err := c.chromaClient.Close(); err != nil {
			return fmt.Errorf("failed to close ChromaDB client: %w", err)
		}
	}
	return nil
}