package query

import (
	"context"
	"reflect"
	"testing"
)

func TestScanFilterEQInt64(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	filter := &ScalarFilter{
		Conditions: []*ScalarCondition{
			{Column: "id", Operator: OpEQ, Value: int64(5)},
		},
	}
	rowIDs, err := env.sf.Filter(ctx, env.manifest, filter)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}
	want := []int64{5}
	if !reflect.DeepEqual(rowIDs, want) {
		t.Errorf("expected %v, got %v", want, rowIDs)
	}
}

func TestScanFilterGTFloat32(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	filter := &ScalarFilter{
		Conditions: []*ScalarCondition{
			{Column: "score", Operator: OpGT, Value: float32(10.0)},
		},
	}
	rowIDs, err := env.sf.Filter(ctx, env.manifest, filter)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}
	want := []int64{21, 22, 23, 24, 25, 26, 27, 28, 29}
	if !reflect.DeepEqual(rowIDs, want) {
		t.Errorf("expected %v, got %v", want, rowIDs)
	}
}

func TestScanFilterInString(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	filter := &ScalarFilter{
		Conditions: []*ScalarCondition{
			{Column: "category", Operator: OpIn, Value: []string{"a"}},
		},
	}
	rowIDs, err := env.sf.Filter(ctx, env.manifest, filter)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}
	want := []int64{0, 3, 6, 9, 12, 15, 18, 21, 24, 27}
	if !reflect.DeepEqual(rowIDs, want) {
		t.Errorf("expected %v, got %v", want, rowIDs)
	}
}

func TestScanFilterMultipleAnd(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	filter := &ScalarFilter{
		Conditions: []*ScalarCondition{
			{Column: "id", Operator: OpGE, Value: int64(5)},
			{Column: "category", Operator: OpEQ, Value: "a"},
		},
	}
	rowIDs, err := env.sf.Filter(ctx, env.manifest, filter)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}
	want := []int64{6, 9, 12, 15, 18, 21, 24, 27}
	if !reflect.DeepEqual(rowIDs, want) {
		t.Errorf("expected %v, got %v", want, rowIDs)
	}
}

func TestScanFilterNoMatch(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	filter := &ScalarFilter{
		Conditions: []*ScalarCondition{
			{Column: "id", Operator: OpEQ, Value: int64(100)},
		},
	}
	rowIDs, err := env.sf.Filter(ctx, env.manifest, filter)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}
	if len(rowIDs) != 0 {
		t.Errorf("expected empty result, got %v", rowIDs)
	}
}

func TestScanFilterBetweenInt64(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	filter := &ScalarFilter{
		Conditions: []*ScalarCondition{
			{Column: "id", Operator: OpBetween, Value: []int64{3, 5}},
		},
	}
	rowIDs, err := env.sf.Filter(ctx, env.manifest, filter)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}
	want := []int64{3, 4, 5}
	if !reflect.DeepEqual(rowIDs, want) {
		t.Errorf("expected %v, got %v", want, rowIDs)
	}
}

func TestScanFilterNilFilter(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	rowIDs, err := env.sf.Filter(ctx, env.manifest, nil)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}
	if rowIDs != nil {
		t.Errorf("expected nil for nil filter, got %v", rowIDs)
	}
}

func TestScanFilterColumnNotFound(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	filter := &ScalarFilter{
		Conditions: []*ScalarCondition{
			{Column: "nonexistent", Operator: OpEQ, Value: int64(1)},
		},
	}
	if _, err := env.sf.Filter(ctx, env.manifest, filter); err == nil {
		t.Error("expected error for non-existent column")
	}
}

func TestMatchesConditionEQ(t *testing.T) {
	cond := &ScalarCondition{Column: "id", Operator: OpEQ, Value: int64(5)}
	if !matchesCondition(int64(5), cond) {
		t.Error("expected true for EQ match")
	}
	if matchesCondition(int64(6), cond) {
		t.Error("expected false for EQ non-match")
	}
}

func TestMatchesConditionNE(t *testing.T) {
	cond := &ScalarCondition{Column: "id", Operator: OpNE, Value: int64(5)}
	if !matchesCondition(int64(3), cond) {
		t.Error("expected true for NE non-match")
	}
	if matchesCondition(int64(5), cond) {
		t.Error("expected false for NE match")
	}
}

func TestMatchesConditionLT(t *testing.T) {
	cond := &ScalarCondition{Column: "score", Operator: OpLT, Value: float32(10)}
	if !matchesCondition(float32(5), cond) {
		t.Error("expected true for LT")
	}
	if matchesCondition(float32(15), cond) {
		t.Error("expected false for LT")
	}
}

func TestMatchesConditionIn(t *testing.T) {
	cond := &ScalarCondition{Column: "cat", Operator: OpIn, Value: []string{"a", "b"}}
	if !matchesCondition("a", cond) {
		t.Error("expected true for In match")
	}
	if matchesCondition("c", cond) {
		t.Error("expected false for In non-match")
	}
}

func TestMatchesConditionBetween(t *testing.T) {
	cond := &ScalarCondition{Column: "id", Operator: OpBetween, Value: []int{1, 10}}
	if !matchesCondition(int64(5), cond) {
		t.Error("expected true for Between")
	}
	if matchesCondition(int64(15), cond) {
		t.Error("expected false for Between")
	}
}

func TestMatchesConditionStringLTUnsupported(t *testing.T) {
	cond := &ScalarCondition{Column: "cat", Operator: OpLT, Value: "b"}
	if matchesCondition("a", cond) {
		t.Error("expected false for string LT (unsupported)")
	}
}
