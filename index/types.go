package index

// DistanceMetric represents distance/similarity metrics for vector search.
type DistanceMetric int32

const (
	// DistanceCosine is 1 - cosine_similarity (0 = identical, 2 = opposite).
	DistanceCosine DistanceMetric = 1
	// DistanceEuclidean is the L2 distance.
	DistanceEuclidean DistanceMetric = 2
	// DistanceDotProduct returns the negative dot product (smaller = more similar).
	DistanceDotProduct DistanceMetric = 3
)

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
type SearchResult struct {
	RowID int64
	Score float64
}

// IndexStats holds index build statistics.
type IndexStats struct {
	NumVectors    int64
	NumPartitions int
	BuildTimeMs   int64
	IndexType     IndexType
}
