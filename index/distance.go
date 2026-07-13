package index

import (
	"fmt"
	"math"
	"sort"
)

// Distance computes the distance between two vectors using the given metric.
// For Cosine: returns 1 - cosine_similarity (0 = identical, 2 = opposite).
// For Euclidean: returns the L2 distance.
// For DotProduct: returns the negative dot product (so smaller = more similar).
func Distance(a, b []float32, metric DistanceMetric) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("index: vector length mismatch: %d vs %d", len(a), len(b))
	}
	switch metric {
	case DistanceCosine:
		var dot, normA, normB float64
		for i := range a {
			af := float64(a[i])
			bf := float64(b[i])
			dot += af * bf
			normA += af * af
			normB += bf * bf
		}
		if normA == 0 || normB == 0 {
			return 0, fmt.Errorf("index: zero-norm vector in cosine distance")
		}
		sim := dot / (math.Sqrt(normA) * math.Sqrt(normB))
		return 1 - sim, nil
	case DistanceEuclidean:
		var sum float64
		for i := range a {
			d := float64(a[i]) - float64(b[i])
			sum += d * d
		}
		return math.Sqrt(sum), nil
	case DistanceDotProduct:
		var dot float64
		for i := range a {
			dot += float64(a[i]) * float64(b[i])
		}
		return -dot, nil
	default:
		return 0, fmt.Errorf("index: unknown distance metric: %d", metric)
	}
}

// Distances computes distances from a query vector to multiple vectors.
func Distances(query []float32, vectors [][]float32, metric DistanceMetric) ([]float64, error) {
	result := make([]float64, len(vectors))
	for i, v := range vectors {
		d, err := Distance(query, v, metric)
		if err != nil {
			return nil, fmt.Errorf("index: %w", err)
		}
		result[i] = d
	}
	return result, nil
}

// TopK selects the k smallest-distance results from a list of (rowID, distance) pairs.
// Returns results sorted by score ascending. It does not mutate the input slice.
func TopK(results []SearchResult, k int) []SearchResult {
	if k <= 0 || len(results) == 0 {
		return []SearchResult{}
	}
	cp := make([]SearchResult, len(results))
	copy(cp, results)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].Score < cp[j].Score
	})
	if k > len(cp) {
		k = len(cp)
	}
	return cp[:k]
}
