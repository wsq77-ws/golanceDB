package index

import (
	"math"
	"testing"
)

func TestDistance_Cosine(t *testing.T) {
	tests := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"identical", []float32{1, 2, 3}, []float32{1, 2, 3}, 0},
		{"orthogonal", []float32{1, 0, 0}, []float32{0, 1, 0}, 1},
		{"opposite", []float32{1, 0, 0}, []float32{-1, 0, 0}, 2},
	}
	for _, tc := range tests {
		got, err := Distance(tc.a, tc.b, DistanceCosine)
		if err != nil {
			t.Fatalf("%s: Distance error: %v", tc.name, err)
		}
		if math.Abs(got-tc.want) > 1e-6 {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDistance_Euclidean(t *testing.T) {
	got, err := Distance([]float32{0, 0}, []float32{3, 4}, DistanceEuclidean)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-5) > 1e-6 {
		t.Fatalf("got %v, want 5", got)
	}

	got, err = Distance([]float32{1, 2, 3}, []float32{1, 2, 3}, DistanceEuclidean)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got) > 1e-6 {
		t.Fatalf("identical got %v, want 0", got)
	}
}

func TestDistance_DotProduct(t *testing.T) {
	got, err := Distance([]float32{1, 2, 3}, []float32{4, 5, 6}, DistanceDotProduct)
	if err != nil {
		t.Fatal(err)
	}
	want := -float64(1*4 + 2*5 + 3*6) // -32
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDistance_MismatchedLength(t *testing.T) {
	if _, err := Distance([]float32{1, 2}, []float32{1, 2, 3}, DistanceEuclidean); err == nil {
		t.Fatal("expected error for mismatched lengths, got nil")
	}
}

func TestDistance_UnknownMetric(t *testing.T) {
	if _, err := Distance([]float32{1}, []float32{1}, DistanceMetric(99)); err == nil {
		t.Fatal("expected error for unknown metric, got nil")
	}
}

func TestDistances_Batch(t *testing.T) {
	query := []float32{1, 0}
	vecs := [][]float32{{1, 0}, {0, 1}, {-1, 0}}
	got, err := Distances(query, vecs, DistanceCosine)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d distances, want 3", len(got))
	}
	want := []float64{0, 1, 2}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-6 {
			t.Fatalf("got[%d]=%v, want %v", i, got[i], want[i])
		}
	}
}

func TestDistances_LengthMismatch(t *testing.T) {
	if _, err := Distances([]float32{1, 2}, [][]float32{{1, 2, 3}}, DistanceEuclidean); err == nil {
		t.Fatal("expected error for length mismatch, got nil")
	}
}

func TestTopK_Unsorted(t *testing.T) {
	results := []SearchResult{
		{RowID: 0, Score: 3},
		{RowID: 1, Score: 1},
		{RowID: 2, Score: 2},
		{RowID: 3, Score: 0},
	}
	got := TopK(results, 2)
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
	if got[0].RowID != 3 || got[1].RowID != 1 {
		t.Fatalf("results not sorted/correct: %+v", got)
	}
}

func TestTopK_KGreaterThanLen(t *testing.T) {
	results := []SearchResult{
		{RowID: 0, Score: 3},
		{RowID: 1, Score: 1},
	}
	got := TopK(results, 5)
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
	if got[0].RowID != 1 || got[1].RowID != 0 {
		t.Fatalf("not sorted ascending: %+v", got)
	}
}

func TestTopK_KZero(t *testing.T) {
	got := TopK([]SearchResult{{RowID: 0, Score: 1}}, 0)
	if len(got) != 0 {
		t.Fatalf("got %d results, want 0", len(got))
	}
}

func TestTopK_Empty(t *testing.T) {
	got := TopK(nil, 3)
	if len(got) != 0 {
		t.Fatalf("got %d results, want 0", len(got))
	}
}
