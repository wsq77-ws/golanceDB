package index

import (
	"context"
	"math"
	"sort"
	"testing"

	"github.com/glancedb/glancedb/storage"
)

// makeVectors generates n deterministic distinct vectors of the given dimension.
func makeVectors(n, dim int) []VectorRecord {
	vectors := make([]VectorRecord, n)
	for i := 0; i < n; i++ {
		v := make([]float32, dim)
		for j := 0; j < dim; j++ {
			v[j] = float32(i*dim + j)
		}
		vectors[i] = VectorRecord{RowID: int64(i), Vector: v}
	}
	return vectors
}

// compareResults compares two SearchResult slices as sets (by RowID), with score tolerance.
func compareResults(t *testing.T, a, b []SearchResult) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("length mismatch: %d vs %d", len(a), len(b))
	}
	ca := make([]SearchResult, len(a))
	copy(ca, a)
	cb := make([]SearchResult, len(b))
	copy(cb, b)
	sort.Slice(ca, func(i, j int) bool { return ca[i].RowID < ca[j].RowID })
	sort.Slice(cb, func(i, j int) bool { return cb[i].RowID < cb[j].RowID })
	for i := range ca {
		if ca[i].RowID != cb[i].RowID {
			t.Fatalf("pos %d: RowID %d vs %d", i, ca[i].RowID, cb[i].RowID)
		}
		if math.Abs(ca[i].Score-cb[i].Score) > 1e-5 {
			t.Fatalf("pos %d: score %v vs %v", i, ca[i].Score, cb[i].Score)
		}
	}
}

func TestIVFFlat_Build(t *testing.T) {
	vectors := makeVectors(100, 128)
	idx := NewIVFFlatIndex(10, DistanceEuclidean)
	if err := idx.Build(context.Background(), vectors); err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if idx.Type() != IndexTypeIVFFlat {
		t.Fatalf("Type = %v, want %v", idx.Type(), IndexTypeIVFFlat)
	}
}

func TestIVFFlat_SearchK1ReturnsSelf(t *testing.T) {
	vectors := makeVectors(100, 128)
	idx := NewIVFFlatIndex(10, DistanceEuclidean)
	if err := idx.Build(context.Background(), vectors); err != nil {
		t.Fatal(err)
	}
	query := vectors[7].Vector
	res, err := idx.Search(context.Background(), query, 1, DistanceEuclidean)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1", len(res))
	}
	if res[0].RowID != 7 {
		t.Fatalf("top-1 RowID = %d, want 7", res[0].RowID)
	}
	if math.Abs(res[0].Score) > 1e-5 {
		t.Fatalf("self-distance = %v, want 0", res[0].Score)
	}
}

func TestIVFFlat_SearchFindsSelf_EachMetric(t *testing.T) {
	vectors := makeVectors(100, 128)
	for _, m := range []DistanceMetric{DistanceCosine, DistanceEuclidean} {
		idx := NewIVFFlatIndex(10, m)
		if err := idx.Build(context.Background(), vectors); err != nil {
			t.Fatalf("metric %v: Build failed: %v", m, err)
		}
		query := vectors[42].Vector
		res, err := idx.Search(context.Background(), query, 1, m)
		if err != nil {
			t.Fatalf("metric %v: Search failed: %v", m, err)
		}
		if len(res) != 1 || res[0].RowID != 42 {
			t.Fatalf("metric %v: top-1 = %+v, want RowID 42", m, res)
		}
	}
}

func TestIVFFlat_SearchMatchesBruteForce_AllMetrics(t *testing.T) {
	vectors := makeVectors(100, 128)
	for _, m := range []DistanceMetric{DistanceCosine, DistanceEuclidean, DistanceDotProduct} {
		// 4 partitions with default nProbes = min(4, 10) = 4 scans all vectors,
		// so IVF results must match brute force exactly.
		ivf := NewIVFFlatIndex(4, m)
		if err := ivf.Build(context.Background(), vectors); err != nil {
			t.Fatalf("metric %v: ivf Build failed: %v", m, err)
		}
		flat := NewFlatIndex(m)
		if err := flat.Build(context.Background(), vectors); err != nil {
			t.Fatalf("metric %v: flat Build failed: %v", m, err)
		}
		query := vectors[15].Vector
		ivfRes, err := ivf.Search(context.Background(), query, 5, m)
		if err != nil {
			t.Fatalf("metric %v: ivf Search failed: %v", m, err)
		}
		flatRes, err := flat.Search(context.Background(), query, 5, m)
		if err != nil {
			t.Fatalf("metric %v: flat Search failed: %v", m, err)
		}
		compareResults(t, ivfRes, flatRes)
	}
}

func TestIVFFlat_SaveLoadRoundtrip(t *testing.T) {
	vectors := makeVectors(100, 128)
	orig := NewIVFFlatIndex(10, DistanceEuclidean)
	if err := orig.Build(context.Background(), vectors); err != nil {
		t.Fatal(err)
	}
	store := storage.NewLocalFS(t.TempDir())
	path := "idx/ivf.json"

	if err := orig.Save(context.Background(), store, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	exists, err := store.Exists(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("saved file does not exist")
	}

	loaded := NewIVFFlatIndex(10, DistanceEuclidean)
	if err := loaded.Load(context.Background(), store, path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	query := vectors[20].Vector
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

func TestIVFFlat_BuildEmpty(t *testing.T) {
	idx := NewIVFFlatIndex(4, DistanceEuclidean)
	if err := idx.Build(context.Background(), nil); err == nil {
		t.Fatal("expected error for empty vectors, got nil")
	}
}

func TestIVFFlat_BuildMismatchedDims(t *testing.T) {
	vectors := []VectorRecord{
		{RowID: 0, Vector: []float32{1, 2, 3}},
		{RowID: 1, Vector: []float32{1, 2}},
	}
	idx := NewIVFFlatIndex(2, DistanceEuclidean)
	if err := idx.Build(context.Background(), vectors); err == nil {
		t.Fatal("expected error for mismatched dimensions, got nil")
	}
}

func TestIVFFlat_Stats(t *testing.T) {
	vectors := makeVectors(100, 128)
	idx := NewIVFFlatIndex(10, DistanceEuclidean)
	if err := idx.Build(context.Background(), vectors); err != nil {
		t.Fatal(err)
	}
	s := idx.Stats()
	if s.NumVectors != 100 {
		t.Fatalf("NumVectors = %d, want 100", s.NumVectors)
	}
	if s.NumPartitions != 10 {
		t.Fatalf("NumPartitions = %d, want 10", s.NumPartitions)
	}
	if s.IndexType != IndexTypeIVFFlat {
		t.Fatalf("IndexType = %v, want %v", s.IndexType, IndexTypeIVFFlat)
	}
}
