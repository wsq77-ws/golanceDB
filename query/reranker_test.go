package query

import (
	"testing"
)

func TestRerankerMergeTopK(t *testing.T) {
	sources := [][]SearchResult{
		{
			{RowID: 1, Score: 1.0},
			{RowID: 2, Score: 3.0},
			{RowID: 3, Score: 5.0},
			{RowID: 4, Score: 7.0},
			{RowID: 5, Score: 9.0},
		},
		{
			{RowID: 6, Score: 0.5},
			{RowID: 7, Score: 2.5},
			{RowID: 8, Score: 4.5},
			{RowID: 9, Score: 6.5},
			{RowID: 10, Score: 8.5},
		},
		{
			{RowID: 11, Score: 1.5},
			{RowID: 12, Score: 2.0},
			{RowID: 13, Score: 4.0},
			{RowID: 14, Score: 6.0},
			{RowID: 15, Score: 10.0},
		},
	}
	r := NewReranker()
	merged := r.MergeTopK(sources, 3)
	if len(merged) != 3 {
		t.Fatalf("expected 3 results, got %d", len(merged))
	}
	expected := []int64{6, 1, 11}
	for i, want := range expected {
		if merged[i].RowID != want {
			t.Errorf("index %d: expected RowID %d, got %d", i, want, merged[i].RowID)
		}
	}
}

func TestRerankerMergeTopKEmptySources(t *testing.T) {
	r := NewReranker()
	merged := r.MergeTopK(nil, 3)
	if len(merged) != 0 {
		t.Errorf("expected 0 results, got %d", len(merged))
	}
}

func TestRerankerMergeTopKKEqualsZero(t *testing.T) {
	sources := [][]SearchResult{
		{{RowID: 1, Score: 1.0}, {RowID: 2, Score: 2.0}},
		{{RowID: 3, Score: 0.5}},
	}
	r := NewReranker()
	merged := r.MergeTopK(sources, 0)
	if len(merged) != 3 {
		t.Fatalf("expected 3 results for k=0 (all), got %d", len(merged))
	}
}

func TestRerankerMergeTopKKExceedsTotal(t *testing.T) {
	sources := [][]SearchResult{
		{{RowID: 1, Score: 5.0}, {RowID: 2, Score: 3.0}},
		{{RowID: 3, Score: 1.0}},
	}
	r := NewReranker()
	merged := r.MergeTopK(sources, 100)
	if len(merged) != 3 {
		t.Fatalf("expected 3 results, got %d", len(merged))
	}
	if merged[0].RowID != 3 || merged[1].RowID != 2 || merged[2].RowID != 1 {
		t.Error("results not sorted by score ascending")
	}
}

func TestRerankerMergeTopKSorted(t *testing.T) {
	sources := [][]SearchResult{
		{{RowID: 1, Score: 10.0}, {RowID: 2, Score: 1.0}},
		{{RowID: 3, Score: 5.0}, {RowID: 4, Score: 3.0}},
	}
	r := NewReranker()
	merged := r.MergeTopK(sources, 4)
	for i := 1; i < len(merged); i++ {
		if merged[i-1].Score > merged[i].Score {
			t.Error("merged results not sorted by score ascending")
		}
	}
}
