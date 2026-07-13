package distance

// DistanceMetric represents the distance metric for vector similarity search.
type DistanceMetric int32

const (
	// DistanceCosine is 1 - cosine_similarity (0 = identical, 2 = opposite).
	DistanceCosine DistanceMetric = 1
	// DistanceEuclidean is the L2 distance (0 = identical).
	DistanceEuclidean DistanceMetric = 2
	// DistanceDotProduct is the negative dot product (more negative = more similar).
	DistanceDotProduct DistanceMetric = 3
)

// SearchResult is a single search hit with row ID and distance score.
type SearchResult struct {
	RowID int64
	Score float64
}
