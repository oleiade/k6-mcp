package search

import (
	"context"
	"fmt"
	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	defaultef "github.com/amikos-tech/chroma-go/pkg/embeddings/default_ef"
	"github.com/oleiade/k6-mcp/internal/logging"
	"log/slog"
	"os"
	"time"
)

type EmbeddingSearch struct {
	databaseURL    string
	client         chroma.Client
	collectionName string
	collection     chroma.Collection
}

var _ Search = &EmbeddingSearch{}

func NewEmbeddingSearch() *EmbeddingSearch {
	databaseURL := DefaultChromaURL
	collectionName := DefaultCollectionName

	// Check for explicit environment variable to override default
	if envURL := os.Getenv("K6_MCP_CHROMA_URL"); envURL != "" {
		databaseURL = envURL
	}

	if envCollectionName := os.Getenv("K6_MCP_CHROMA_collection"); envCollectionName != "" {
		collectionName = envCollectionName
	}

	return &EmbeddingSearch{
		databaseURL:    databaseURL,
		collectionName: collectionName,
	}
}

// Search returns up to limit results for the provided MATCH query.
func (e *EmbeddingSearch) Search(ctx context.Context, query string, opts Options) ([]Result, error) {
	logger := logging.WithComponent("search")
	startTime := time.Now()

	logger.DebugContext(ctx, "Starting ChromaDB search",
		slog.String("query_hash", hashQuery(query)),
		slog.Int("max_results", opts.MaxResults),
	)

	// Initialize ChromaDB client and collection if needed
	if err := e.initializeClient(ctx); err != nil {
		logger.ErrorContext(ctx, "Failed to initialize ChromaDB client",
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to initialize ChromaDB client: %w", err)
	}

	// Query the collection using the same approach as chroma-test/main.go
	queryResult, err := e.collection.Query(ctx,
		chroma.WithQueryTexts(query),
		chroma.WithNResults(opts.MaxResults),
	)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to query ChromaDB collection",
			slog.String("error", err.Error()),
			slog.String("collection", e.collectionName),
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
			result.Rank = float64(1.0 - float32(distance))
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

const (
	// DefaultChromaURL is the default ChromaDB server URL.
	DefaultChromaURL      = "http://localhost:8000"
	DefaultCollectionName = "k6_docs"
)

// initializeClient initializes the ChromaDB client and collection if not already done.
func (e *EmbeddingSearch) initializeClient(ctx context.Context) error {
	logger := logging.WithComponent("search")
	startTime := time.Now()

	if e.client != nil && e.collection != nil {
		logger.DebugContext(ctx, "ChromaDB client already initialized")
		return nil // Already initialized
	}

	logger.DebugContext(ctx, "Initializing ChromaDB client",
		slog.String("url", e.databaseURL),
		slog.String("collection", e.collectionName),
	)

	// Create ChromaDB client with custom URL if specified
	var client chroma.Client
	var err error

	if e.databaseURL != DefaultChromaURL {
		// Custom URL specified
		client, err = chroma.NewHTTPClient(chroma.WithBaseURL(e.databaseURL))
	} else {
		// Use default URL
		client, err = chroma.NewHTTPClient()
	}

	if err != nil {
		logger.ErrorContext(ctx, "Failed to create ChromaDB client",
			slog.String("error", err.Error()),
			slog.String("url", e.databaseURL),
		)
		return fmt.Errorf("failed to create ChromaDB client: %w", err)
	}

	e.client = client
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
	collection, err := client.GetCollection(ctx, e.collectionName, chroma.WithEmbeddingFunctionGet(ef))
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get collection",
			slog.String("error", err.Error()),
			slog.String("collection", e.collectionName),
		)
		return fmt.Errorf("failed to get collection '%s': %w", e.collectionName, err)
	}

	e.collection = collection

	logger.InfoContext(ctx, "ChromaDB client initialized successfully",
		slog.String("collection", e.collectionName),
		slog.Duration("duration", time.Since(startTime)),
	)

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
