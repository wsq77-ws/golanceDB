package index

import "github.com/glancedb/glancedb/distance"

// VectorRecord pairs a row ID with its vector.
type VectorRecord struct {
	RowID  int64
	Vector []float32
}

// IndexType represents the kind of ANN index.
type IndexType int32

const (
	IndexTypeIVFPQ   IndexType = 1
	IndexTypeIVFFlat IndexType = 2
	IndexTypeHNSW    IndexType = 3
	IndexTypeFlat    IndexType = 4
)

// SearchResult is a single search hit.
type SearchResult = distance.SearchResult

// DistanceMetric is re-exported from the distance package for convenience.
type DistanceMetric = distance.DistanceMetric

const (
	DistanceCosine     = distance.DistanceCosine
	DistanceEuclidean  = distance.DistanceEuclidean
	DistanceDotProduct = distance.DistanceDotProduct
)

// IndexStats holds index build statistics.
type IndexStats struct {
	NumVectors    int64
	NumPartitions int
	BuildTimeMs   int64
	IndexType     IndexType
}
