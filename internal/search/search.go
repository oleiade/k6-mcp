// Package search provides k6 documentation search functionality using embeddings.
package search

import (
	"context"
	"io"
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
	CollectionName string `json:"collection_name"`
}

// Search defines the interface for documentation search implementations.
type Search interface {
	// Search searches the documentation for content similar to the query.
	Search(ctx context.Context, query string) ([]Result, error)
	
	// Close implements io.Closer to release resources.
	io.Closer
}

const (
	defaultMaxResults     = 5
	defaultCollectionName = "k6_docs"
)

// DefaultOptions returns default search configuration.
func DefaultOptions() *Options {
	return &Options{
		MaxResults:     defaultMaxResults,
		CollectionName: defaultCollectionName,
	}
}

// BackendType represents the type of search backend to use.
type BackendType string

const (
	// BackendChroma represents ChromaDB backend.
	BackendChroma BackendType = "chroma"
)

// NewSearch creates a new search client based on the backend type.
func NewSearch(backend BackendType, options *Options) (Search, error) {
	if options == nil {
		options = DefaultOptions()
	}
	
	switch backend {
	case BackendChroma:
		return NewChromaSearch(options)
	default:
		return NewChromaSearch(options) // Default to Chroma for now
	}
}