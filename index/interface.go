package index

import (
	"context"

	"github.com/glancedb/glancedb/storage"
)

// Index is the extensible interface for all ANN index implementations.
// Future index types (HNSW, IVFPQ, DiskANN) implement this interface.
type Index interface {
	// Build constructs the index from the given vectors.
	Build(ctx context.Context, vectors []VectorRecord) error

	// Search returns the top-k nearest neighbors for the query vector.
	Search(ctx context.Context, query []float32, k int, metric DistanceMetric) ([]SearchResult, error)

	// Stats returns build statistics.
	Stats() IndexStats

	// Type returns the index type.
	Type() IndexType

	// Save persists the index to the given path via the ObjectStore.
	Save(ctx context.Context, store storage.ObjectStore, path string) error

	// Load reads the index from the given path via the ObjectStore.
	Load(ctx context.Context, store storage.ObjectStore, path string) error
}
