package query

import (
	"context"
	"testing"
)

func TestHybridSearchVectorOnly(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	q := &Query{
		Vector: &VectorQuery{
			Vector: []float32{4, 5, 6, 7},
			Column: "embedding",
			Metric: DistanceEuclidean,
		},
		Limit: 5,
	}
	results, err := env.hybrid.Search(ctx, env.manifest, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	if results[0].RowID != 4 {
		t.Errorf("expected nearest RowID 4, got %d", results[0].RowID)
	}
	if results[0].Score != 0 {
		t.Errorf("expected distance 0, got %v", results[0].Score)
	}
}

func TestHybridSearchFilterOnly(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	q := &Query{
		Filter: &ScalarFilter{
			Conditions: []*ScalarCondition{
				{Column: "category", Operator: OpEQ, Value: "a"},
			},
		},
		Limit: 0,
	}
	results, err := env.hybrid.Search(ctx, env.manifest, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Score != 0 {
			t.Errorf("expected Score 0 for filter-only, got %v", r.Score)
		}
	}
}

func TestHybridSearchFilterFirst(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	q := &Query{
		Vector: &VectorQuery{
			Vector: []float32{4, 5, 6, 7},
			Column: "embedding",
			Metric: DistanceEuclidean,
		},
		Filter: &ScalarFilter{
			Conditions: []*ScalarCondition{
				{Column: "category", Operator: OpEQ, Value: "a"},
			},
		},
		Limit: 0,
	}
	results, err := env.hybrid.Search(ctx, env.manifest, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	categoryARows := []int64{0, 3, 6, 9, 12, 15, 18, 21, 24, 27}
	if len(results) != len(categoryARows) {
		t.Fatalf("expected %d results, got %d", len(categoryARows), len(results))
	}

	seen := make(map[int64]bool)
	for _, r := range results {
		seen[r.RowID] = true
	}
	for _, id := range categoryARows {
		if !seen[id] {
			t.Errorf("expected RowID %d in results", id)
		}
	}

	for i := 1; i < len(results); i++ {
		if results[i-1].Score > results[i].Score {
			t.Error("results not sorted by score ascending")
		}
	}
}

func TestHybridSearchSearchFirst(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	q := &Query{
		Vector: &VectorQuery{
			Vector: []float32{4, 5, 6, 7},
			Column: "embedding",
			Metric: DistanceEuclidean,
		},
		Filter: &ScalarFilter{
			Conditions: []*ScalarCondition{
				{Column: "category", Operator: OpEQ, Value: "a"},
			},
		},
		Limit: 0,
	}
	results, err := env.hybrid.SearchWithStrategy(ctx, env.manifest, q, StrategySearchFirst)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	categoryARows := []int64{0, 3, 6, 9, 12, 15, 18, 21, 24, 27}
	if len(results) != len(categoryARows) {
		t.Fatalf("expected %d results, got %d", len(categoryARows), len(results))
	}

	seen := make(map[int64]bool)
	for _, r := range results {
		seen[r.RowID] = true
	}
	for _, id := range categoryARows {
		if !seen[id] {
			t.Errorf("expected RowID %d in results", id)
		}
	}
}

func TestHybridSearchFilteredIsSubset(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	vectorQuery := &VectorQuery{
		Vector: []float32{15, 16, 17, 18},
		Column: "embedding",
		Metric: DistanceEuclidean,
	}

	allResults, err := env.bf.Search(ctx, env.manifest, vectorQuery)
	if err != nil {
		t.Fatalf("brute force Search failed: %v", err)
	}
	allSet := make(map[int64]bool)
	for _, r := range allResults {
		allSet[r.RowID] = true
	}

	q := &Query{
		Vector: vectorQuery,
		Filter: &ScalarFilter{
			Conditions: []*ScalarCondition{
				{Column: "id", Operator: OpLT, Value: int64(10)},
			},
		},
		Limit: 0,
	}
	filtered, err := env.hybrid.Search(ctx, env.manifest, q)
	if err != nil {
		t.Fatalf("hybrid Search failed: %v", err)
	}

	for _, r := range filtered {
		if !allSet[r.RowID] {
			t.Errorf("filtered RowID %d not in full result set", r.RowID)
		}
	}
}

func TestHybridSearchFilterFirstEmptyFilter(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	q := &Query{
		Vector: &VectorQuery{
			Vector: []float32{4, 5, 6, 7},
			Column: "embedding",
			Metric: DistanceEuclidean,
		},
		Filter: &ScalarFilter{
			Conditions: []*ScalarCondition{
				{Column: "id", Operator: OpEQ, Value: int64(100)},
			},
		},
	}
	results, err := env.hybrid.Search(ctx, env.manifest, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-matching filter, got %d", len(results))
	}
}

func TestHybridSearchNilQuery(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	q := &Query{}
	results, err := env.hybrid.Search(ctx, env.manifest, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil for empty query, got %v", results)
	}
}
