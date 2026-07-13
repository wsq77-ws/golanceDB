package query

import (
	"context"
	"testing"

	"github.com/glancedb/glancedb/distance"
	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/storage"
	"github.com/glancedb/glancedb/table"
)

const (
	testRowsPerFrag = 10
	testNumFrags    = 3
	testDim         = 4
)

// testEnv holds a fully wired test environment with 3 fragments of 10 rows each.
type testEnv struct {
	reader   *table.FragmentReader
	schema   *table.Schema
	manifest *table.Manifest
	bf       *BruteForceSearch
	sf       *ScanFilter
	hybrid   *HybridSearch
}

// embeddingFor returns the embedding vector for global row r.
func embeddingFor(r int) []float32 {
	return []float32{float32(r), float32(r + 1), float32(r + 2), float32(r + 3)}
}

// setupTestEnv creates a test environment with known data.
// Schema: id(int64), score(float32), embedding(FixedSizeList dim=4), category(string).
// 3 fragments x 10 rows. Global row r:
//
//	id=r, score=r*0.5, embedding=[r,r+1,r+2,r+3], category=a/b/c by r%3.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	store := storage.NewLocalFS(dir)
	schema := table.NewSchema([]*table.Field{
		{Name: "id", Type: encode.TypeInt64},
		{Name: "score", Type: encode.TypeFloat32},
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: testDim},
		{Name: "category", Type: encode.TypeString},
	})

	var fragments []*table.Fragment
	for f := 0; f < testNumFrags; f++ {
		fw := table.NewFragmentWriter(store, schema, int32(f), "tbl", encode.CompressionNone)
		batch := table.NewRecordBatch(schema, int64(testRowsPerFrag))
		ids := make([]int64, testRowsPerFrag)
		scores := make([]float32, testRowsPerFrag)
		embeddings := make([]float32, testRowsPerFrag*testDim)
		cats := make([]string, testRowsPerFrag)
		for i := 0; i < testRowsPerFrag; i++ {
			r := f*testRowsPerFrag + i
			ids[i] = int64(r)
			scores[i] = float32(r) * 0.5
			emb := embeddingFor(r)
			copy(embeddings[i*testDim:], emb)
			switch r % 3 {
			case 0:
				cats[i] = "a"
			case 1:
				cats[i] = "b"
			default:
				cats[i] = "c"
			}
		}
		batch.SetColumn(0, ids)
		batch.SetColumn(1, scores)
		batch.SetColumn(2, embeddings)
		batch.SetColumn(3, cats)
		if err := fw.WriteBatch(ctx, batch); err != nil {
			t.Fatalf("WriteBatch fragment %d failed: %v", f, err)
		}
		frag, err := fw.Finish()
		if err != nil {
			t.Fatalf("Finish fragment %d failed: %v", f, err)
		}
		fragments = append(fragments, frag)
	}

	manifest := table.NewManifest(1, schema)
	manifest.Fragments = fragments
	manifest.MaxFragmentID = int32(testNumFrags - 1)

	reader := table.NewFragmentReader(store, schema, encode.CompressionNone)
	bf := NewBruteForceSearch(reader, schema)
	sf := NewScanFilter(reader, schema)
	hybrid := NewHybridSearch(bf, sf)

	return &testEnv{
		reader:   reader,
		schema:   schema,
		manifest: manifest,
		bf:       bf,
		sf:       sf,
		hybrid:   hybrid,
	}
}

func TestBruteForceSearchEuclidean(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	query := &VectorQuery{
		Vector: []float32{4, 5, 6, 7},
		Column: "embedding",
		Metric: DistanceEuclidean,
	}
	results, err := env.bf.Search(ctx, env.manifest, query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != testRowsPerFrag*testNumFrags {
		t.Fatalf("expected %d results, got %d", testRowsPerFrag*testNumFrags, len(results))
	}
	if results[0].RowID != 4 {
		t.Errorf("expected nearest RowID 4, got %d", results[0].RowID)
	}
	if results[0].Score != 0 {
		t.Errorf("expected distance 0 for exact match, got %v", results[0].Score)
	}
}

func TestBruteForceSearchExactMatchEuclidean(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	target := embeddingFor(17)
	query := &VectorQuery{
		Vector: target,
		Column: "embedding",
		Metric: DistanceEuclidean,
	}
	results, err := env.bf.Search(ctx, env.manifest, query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if results[0].RowID != 17 {
		t.Errorf("expected nearest RowID 17, got %d", results[0].RowID)
	}
	if results[0].Score != 0 {
		t.Errorf("expected distance 0 for exact match, got %v", results[0].Score)
	}
}

func TestBruteForceSearchCosine(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	query := &VectorQuery{
		Vector: []float32{4, 5, 6, 7},
		Column: "embedding",
		Metric: DistanceCosine,
	}
	results, err := env.bf.Search(ctx, env.manifest, query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if results[0].RowID != 4 {
		t.Errorf("expected nearest RowID 4, got %d", results[0].RowID)
	}
	if results[0].Score != 0 {
		t.Errorf("expected cosine distance 0 for exact match, got %v", results[0].Score)
	}
}

func TestBruteForceSearchDotProduct(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	query := &VectorQuery{
		Vector: []float32{4, 5, 6, 7},
		Column: "embedding",
		Metric: DistanceDotProduct,
	}
	results, err := env.bf.Search(ctx, env.manifest, query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if results[0].RowID != 29 {
		t.Errorf("expected nearest RowID 29 (largest dot product), got %d", results[0].RowID)
	}
}

func TestBruteForceSearchWithLimit(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	query := &VectorQuery{
		Vector: []float32{4, 5, 6, 7},
		Column: "embedding",
		Metric: DistanceEuclidean,
	}
	results, err := env.bf.Search(ctx, env.manifest, query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	limited := distance.TopK(results, 3)
	if len(limited) != 3 {
		t.Fatalf("expected 3 results, got %d", len(limited))
	}
	if limited[0].Score > limited[1].Score || limited[1].Score > limited[2].Score {
		t.Error("results not sorted by score ascending")
	}
	if limited[0].RowID != 4 {
		t.Errorf("expected first result RowID 4, got %d", limited[0].RowID)
	}
}

func TestBruteForceSearchSortedAscending(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	query := &VectorQuery{
		Vector: []float32{15, 16, 17, 18},
		Column: "embedding",
		Metric: DistanceEuclidean,
	}
	results, err := env.bf.Search(ctx, env.manifest, query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	for i := 1; i < len(results); i++ {
		if results[i-1].Score > results[i].Score {
			t.Errorf("results not sorted at index %d: %v > %v", i, results[i-1].Score, results[i].Score)
		}
	}
}

func TestBruteForceSearchColumnNotFound(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	query := &VectorQuery{
		Vector: []float32{1, 2, 3, 4},
		Column: "nonexistent",
		Metric: DistanceEuclidean,
	}
	if _, err := env.bf.Search(ctx, env.manifest, query); err == nil {
		t.Error("expected error for non-existent column")
	}
}

func TestBruteForceSearchDimensionMismatch(t *testing.T) {
	ctx := context.Background()
	env := setupTestEnv(t)

	query := &VectorQuery{
		Vector: []float32{1, 2, 3},
		Column: "embedding",
		Metric: DistanceEuclidean,
	}
	if _, err := env.bf.Search(ctx, env.manifest, query); err == nil {
		t.Error("expected error for vector dimension mismatch")
	}
}
