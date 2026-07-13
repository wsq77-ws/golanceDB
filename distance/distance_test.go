package distance

import (
	"math"
	"testing"
)

func TestCosineDistance(t *testing.T) {
	// Identical vectors: distance = 0.
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	d, err := Distance(a, b, DistanceCosine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 0 {
		t.Fatalf("expected 0 for identical vectors, got %f", d)
	}

	// Orthogonal vectors: distance = 1.
	a = []float32{1, 0, 0}
	b = []float32{0, 1, 0}
	d, err = Distance(a, b, DistanceCosine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(d-1) > 1e-6 {
		t.Fatalf("expected ~1 for orthogonal vectors, got %f", d)
	}

	// Opposite vectors: distance = 2.
	a = []float32{1, 0, 0}
	b = []float32{-1, 0, 0}
	d, err = Distance(a, b, DistanceCosine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(d-2) > 1e-6 {
		t.Fatalf("expected ~2 for opposite vectors, got %f", d)
	}
}

func TestEuclideanDistance(t *testing.T) {
	// 3-4-5 triangle.
	a := []float32{0, 0}
	b := []float32{3, 4}
	d, err := Distance(a, b, DistanceEuclidean)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(d-5) > 1e-6 {
		t.Fatalf("expected 5 for 3-4-5 triangle, got %f", d)
	}

	// Same point.
	a = []float32{1, 2, 3}
	b = []float32{1, 2, 3}
	d, err = Distance(a, b, DistanceEuclidean)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 0 {
		t.Fatalf("expected 0 for same point, got %f", d)
	}
}

func TestDotProductDistance(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{4, 5, 6}
	d, err := Distance(a, b, DistanceDotProduct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// dot = 1*4 + 2*5 + 3*6 = 4 + 10 + 18 = 32, expected = -32.
	expected := -32.0
	if d != expected {
		t.Fatalf("expected %f, got %f", expected, d)
	}
}

func TestDistanceErrorCases(t *testing.T) {
	if _, err := Distance([]float32{1, 2}, []float32{1, 2, 3}, DistanceEuclidean); err == nil {
		t.Fatal("expected error for mismatched lengths")
	}
	if _, err := Distance([]float32{0, 0}, []float32{0, 0}, DistanceEuclidean); err != nil {
		t.Fatal("zero-length vectors should succeed for Euclidean")
	}
	if _, err := Distance([]float32{0, 0}, []float32{1, 1}, DistanceCosine); err == nil {
		t.Fatal("expected error for zero-norm vector in cosine")
	}
	if _, err := Distance([]float32{1}, []float32{1}, DistanceMetric(99)); err == nil {
		t.Fatal("expected error for unknown metric")
	}
}

func TestDistancesBatch(t *testing.T) {
	query := []float32{1, 0}
	vecs := [][]float32{
		{1, 0},  // cos=0
		{0, 1},  // cos=1
		{-1, 0}, // cos=2
	}
	got, err := Distances(query, vecs, DistanceCosine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 distances, got %d", len(got))
	}
	if math.Abs(got[0]) > 1e-6 {
		t.Fatalf("expected ~0 for identical, got %f", got[0])
	}
	if math.Abs(got[1]-1) > 1e-6 {
		t.Fatalf("expected ~1 for orthogonal, got %f", got[1])
	}
	if math.Abs(got[2]-2) > 1e-6 {
		t.Fatalf("expected ~2 for opposite, got %f", got[2])
	}

	// Mismatched length in batch.
	if _, err := Distances([]float32{1, 2}, [][]float32{{1, 2, 3}}, DistanceEuclidean); err == nil {
		t.Fatal("expected error for mismatched batch vector")
	}
}

func TestTopK(t *testing.T) {
	results := []SearchResult{
		{RowID: 0, Score: 3},
		{RowID: 1, Score: 1},
		{RowID: 2, Score: 2},
	}
	got := TopK(results, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].RowID != 1 || got[0].Score != 1 {
		t.Fatalf("expected RowID=1 Score=1 first, got RowID=%d Score=%f", got[0].RowID, got[0].Score)
	}
	if got[1].RowID != 2 || got[1].Score != 2 {
		t.Fatalf("expected RowID=2 Score=2 second, got RowID=%d Score=%f", got[1].RowID, got[1].Score)
	}
}

func TestTopKKGreaterThanLen(t *testing.T) {
	results := []SearchResult{{RowID: 0, Score: 1}}
	got := TopK(results, 5)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
}

func TestTopKKZero(t *testing.T) {
	got := TopK([]SearchResult{{RowID: 0, Score: 1}}, 0)
	if len(got) != 0 {
		t.Fatalf("expected 0 results for k=0, got %d", len(got))
	}
}

func TestTopKEmpty(t *testing.T) {
	got := TopK(nil, 3)
	if len(got) != 0 {
		t.Fatalf("expected 0 results for empty input, got %d", len(got))
	}
}

func TestTopKDoesNotMutateInput(t *testing.T) {
	results := []SearchResult{
		{RowID: 0, Score: 2},
		{RowID: 1, Score: 1},
	}
	_ = TopK(results, 1)
	if results[0].RowID != 0 || results[1].RowID != 1 {
		t.Fatal("TopK mutated the input slice")
	}
}
