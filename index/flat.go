package index

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/glancedb/glancedb/storage"
)

// FlatIndex performs brute-force search over all vectors.
// It is the simplest index and serves as the baseline.
type FlatIndex struct {
	vectors []VectorRecord
	metric  DistanceMetric
	stats   IndexStats
	dim     int
}

var _ Index = (*FlatIndex)(nil)

// NewFlatIndex creates a FlatIndex using the given metric as the default.
func NewFlatIndex(metric DistanceMetric) *FlatIndex {
	return &FlatIndex{metric: metric}
}

// Type returns IndexTypeFlat.
func (idx *FlatIndex) Type() IndexType { return IndexTypeFlat }

// Stats returns build statistics.
func (idx *FlatIndex) Stats() IndexStats { return idx.stats }

// Build stores the vectors for brute-force search.
func (idx *FlatIndex) Build(ctx context.Context, vectors []VectorRecord) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	if len(vectors) == 0 {
		return fmt.Errorf("index: cannot build Flat from empty vectors")
	}
	dim := len(vectors[0].Vector)
	if dim == 0 {
		return fmt.Errorf("index: vector dimension must be > 0")
	}
	for i, v := range vectors {
		if len(v.Vector) != dim {
			return fmt.Errorf("index: vector %d has dimension %d, want %d", i, len(v.Vector), dim)
		}
	}
	idx.vectors = make([]VectorRecord, len(vectors))
	copy(idx.vectors, vectors)
	idx.dim = dim
	idx.stats = IndexStats{
		NumVectors: int64(len(vectors)),
		IndexType:  IndexTypeFlat,
	}
	return nil
}

// Search computes the distance to every vector and returns the top-k results.
func (idx *FlatIndex) Search(ctx context.Context, query []float32, k int, metric DistanceMetric) ([]SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}
	if len(idx.vectors) == 0 {
		return nil, fmt.Errorf("index: index not built or empty")
	}
	if len(query) != idx.dim {
		return nil, fmt.Errorf("index: query dimension %d does not match index dimension %d", len(query), idx.dim)
	}
	if k <= 0 {
		return []SearchResult{}, nil
	}
	results := make([]SearchResult, len(idx.vectors))
	for i, v := range idx.vectors {
		d, err := Distance(query, v.Vector, metric)
		if err != nil {
			return nil, fmt.Errorf("index: %w", err)
		}
		results[i] = SearchResult{RowID: v.RowID, Score: d}
	}
	return TopK(results, k), nil
}

// flatSnapshot is the JSON-serializable form of a FlatIndex.
type flatSnapshot struct {
	Vectors []VectorRecord `json:"vectors"`
	Metric  DistanceMetric `json:"metric"`
	Dim     int            `json:"dim"`
}

// Save persists the index to the given path as JSON via the ObjectStore.
func (idx *FlatIndex) Save(ctx context.Context, store storage.ObjectStore, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	snap := flatSnapshot{
		Vectors: idx.vectors,
		Metric:  idx.metric,
		Dim:     idx.dim,
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	if err := store.Write(ctx, path, data); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	return nil
}

// Load reads the index from the given path via the ObjectStore.
func (idx *FlatIndex) Load(ctx context.Context, store storage.ObjectStore, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	size, err := store.Size(ctx, path)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	data, err := store.Read(ctx, path, 0, size)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	var snap flatSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	idx.vectors = snap.Vectors
	idx.metric = snap.Metric
	idx.dim = snap.Dim
	idx.stats = IndexStats{
		NumVectors: int64(len(snap.Vectors)),
		IndexType:  IndexTypeFlat,
	}
	return nil
}
