package search

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	defaultef "github.com/amikos-tech/chroma-go/pkg/embeddings/default_ef"

	"github.com/oleiade/k6-mcp/internal/logging"
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
	logger := logging.WithComponent("search")
	
	chromaOpts := &ChromaOptions{
		Options: options,
		URL:     DefaultChromaURL,
	}

	// Check for explicit environment variable to override default
	if envURL := os.Getenv("CHROMA_URL"); envURL != "" {
		chromaOpts.URL = envURL
	}

	logger.Debug("Creating ChromaDB search client",
		slog.String("url", chromaOpts.URL),
		slog.String("collection", chromaOpts.CollectionName),
		slog.Int("max_results", chromaOpts.MaxResults),
	)

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
	logger := logging.WithComponent("search")
	startTime := time.Now()
	
	if c.chromaClient != nil && c.collection != nil {
		logger.DebugContext(ctx, "ChromaDB client already initialized")
		return nil // Already initialized
	}

	logger.DebugContext(ctx, "Initializing ChromaDB client",
		slog.String("url", c.options.URL),
		slog.String("collection", c.options.CollectionName),
	)

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
		logger.ErrorContext(ctx, "Failed to create ChromaDB client",
			slog.String("error", err.Error()),
			slog.String("url", c.options.URL),
		)
		return fmt.Errorf("failed to create ChromaDB client: %w", err)
	}

	c.chromaClient = client
	logger.DebugContext(ctx, "ChromaDB client created successfully")

	// Create default embedding function (all-MiniLM-L6-v2)
	// This must work - no fallbacks
	ef, _, err := defaultef.NewDefaultEmbeddingFunction()
	if err != nil {
		logger.ErrorContext(ctx, "Failed to initialize embedding function",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to initialize default embedding function: %w", err)
	}

	logger.DebugContext(ctx, "Embedding function initialized")

	// Get the collection with our embedding function
	collection, err := client.GetCollection(ctx, c.options.CollectionName, chroma.WithEmbeddingFunctionGet(ef))
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get collection",
			slog.String("error", err.Error()),
			slog.String("collection", c.options.CollectionName),
		)
		return fmt.Errorf("failed to get collection '%s': %w", c.options.CollectionName, err)
	}

	c.collection = collection
	
	logger.InfoContext(ctx, "ChromaDB client initialized successfully",
		slog.String("collection", c.options.CollectionName),
		slog.Duration("duration", time.Since(startTime)),
	)
	
	return nil
}

// Search searches the k6 documentation for content similar to the query.
func (c *ChromaSearch) Search(ctx context.Context, query string) ([]Result, error) {
	logger := logging.WithComponent("search")
	startTime := time.Now()
	
	logger.DebugContext(ctx, "Starting ChromaDB search",
		slog.String("query_hash", hashQuery(query)),
		slog.Int("max_results", c.options.MaxResults),
	)

	// Initialize ChromaDB client and collection if needed
	if err := c.initializeChromaClient(ctx); err != nil {
		logger.ErrorContext(ctx, "Failed to initialize ChromaDB client",
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to initialize ChromaDB client: %w", err)
	}

	// Query the collection using the same approach as chroma-test/main.go
	queryResult, err := c.collection.Query(ctx,
		chroma.WithQueryTexts(query),
		chroma.WithNResults(c.options.MaxResults),
	)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to query ChromaDB collection",
			slog.String("error", err.Error()),
			slog.String("collection", c.options.CollectionName),
		)
		return nil, fmt.Errorf("failed to query ChromaDB collection: %w", err)
	}

	// Convert ChromaDB query result to our Result format
	results := make([]Result, 0)

	// Get the document groups from the query result
	documentGroups := queryResult.GetDocumentsGroups()
	if len(documentGroups) == 0 {
		logger.DebugContext(ctx, "No document groups found in query result")
		return results, nil
	}

	// Process the first group (since we only have one query)
	documents := documentGroups[0]
	metadatas := queryResult.GetMetadatasGroups()
	distances := queryResult.GetDistancesGroups()

	logger.DebugContext(ctx, "Processing search results",
		slog.Int("document_count", len(documents)),
		slog.Bool("has_metadata", len(metadatas) > 0),
		slog.Bool("has_distances", len(distances) > 0),
	)

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

	logger.InfoContext(ctx, "ChromaDB search completed",
		slog.Int("result_count", len(results)),
		slog.Duration("duration", time.Since(startTime)),
		slog.Int64("duration_ms", time.Since(startTime).Milliseconds()),
	)

	return results, nil
}

// Close closes the ChromaDB client to release resources.
func (c *ChromaSearch) Close() error {
	logger := logging.WithComponent("search")
	
	if c.chromaClient != nil {
		logger.Debug("Closing ChromaDB client")
		if err := c.chromaClient.Close(); err != nil {
			logger.Error("Failed to close ChromaDB client",
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("failed to close ChromaDB client: %w", err)
		}
		logger.Debug("ChromaDB client closed successfully")
	}
	return nil
}

// hashQuery creates a simple hash of a query for privacy-preserving logging
func hashQuery(query string) string {
	if len(query) == 0 {
		return "empty"
	}
	
	// Simple hash for privacy - just use length and first/last char
	first := string(query[0])
	last := string(query[len(query)-1])
	return fmt.Sprintf("len_%d_%s_%s", len(query), first, last)
}