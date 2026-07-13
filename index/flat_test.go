package index

import (
	"context"
	"math"
	"testing"

	"github.com/glancedb/glancedb/storage"
)

func TestFlat_BuildSearch(t *testing.T) {
	vectors := makeVectors(100, 64)
	idx := NewFlatIndex(DistanceEuclidean)
	if err := idx.Build(context.Background(), vectors); err != nil {
		t.Fatal(err)
	}
	query := vectors[5].Vector
	res, err := idx.Search(context.Background(), query, 1, DistanceEuclidean)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].RowID != 5 {
		t.Fatalf("top-1 = %+v, want RowID 5", res)
	}
	if math.Abs(res[0].Score) > 1e-6 {
		t.Fatalf("self-distance = %v, want 0", res[0].Score)
	}
}

func TestFlat_SearchKGreaterThanN(t *testing.T) {
	vectors := makeVectors(5, 16)
	idx := NewFlatIndex(DistanceEuclidean)
	if err := idx.Build(context.Background(), vectors); err != nil {
		t.Fatal(err)
	}
	res, err := idx.Search(context.Background(), vectors[0].Vector, 20, DistanceEuclidean)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 5 {
		t.Fatalf("got %d results, want 5", len(res))
	}
}

func TestFlat_Type(t *testing.T) {
	idx := NewFlatIndex(DistanceEuclidean)
	if idx.Type() != IndexTypeFlat {
		t.Fatalf("Type = %v, want %v", idx.Type(), IndexTypeFlat)
	}
}

func TestFlat_SaveLoadRoundtrip(t *testing.T) {
	vectors := makeVectors(50, 32)
	orig := NewFlatIndex(DistanceEuclidean)
	if err := orig.Build(context.Background(), vectors); err != nil {
		t.Fatal(err)
	}
	store := storage.NewLocalFS(t.TempDir())
	path := "flat.json"
	if err := orig.Save(context.Background(), store, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := NewFlatIndex(DistanceEuclidean)
	if err := loaded.Load(context.Background(), store, path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	query := vectors[3].Vector
	r1, err := orig.Search(context.Background(), query, 5, DistanceEuclidean)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := loaded.Search(context.Background(), query, 5, DistanceEuclidean)
	if err != nil {
		t.Fatal(err)
	}
	compareResults(t, r1, r2)
}

func TestFlat_BuildEmpty(t *testing.T) {
	idx := NewFlatIndex(DistanceEuclidean)
	if err := idx.Build(context.Background(), nil); err == nil {
		t.Fatal("expected error for empty vectors, got nil")
	}
}
