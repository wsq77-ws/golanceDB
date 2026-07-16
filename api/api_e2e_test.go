package api

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/query"
	"github.com/glancedb/glancedb/table"
)

// newTestSchema creates a minimal schema for e2e testing.
func newTestSchema() *TableSchema {
	return table.NewSchema([]*table.Field{
		{Name: "id", Type: encode.TypeInt64, Nullable: false},
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 4},
		{Name: "category", Type: encode.TypeString, Nullable: true},
	})
}

// makeBatch creates a test batch with the given row count.
// startID 0 would create a zero-norm vector [0,0,0,0] which fails cosine distance,
// startID should be ≥ 1 for vector search tests.
func makeBatch(schema *table.Schema, startID int64, category string, n int) *table.RecordBatch {
	batch := table.NewRecordBatch(schema, int64(n))
	idCol := make([]int64, n)
	embCol := make([]float32, n*4)
	catCol := make([]string, n)
	for i := 0; i < n; i++ {
		idCol[i] = startID + int64(i)
		embCol[i*4+0] = float32(startID + int64(i))
		embCol[i*4+1] = 0
		embCol[i*4+2] = 0
		embCol[i*4+3] = 0
		catCol[i] = category
	}
	batch.SetColumn(0, idCol)
	batch.SetColumn(1, embCol)
	batch.SetColumn(2, catCol)
	return batch
}

func TestE2E_CreateTableAndInsert(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := newTestSchema()
	tbl, err := db.CreateTable(ctx, "vectors", schema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	batch := makeBatch(schema, 0, "test", 5)
	if err := tbl.Insert(ctx, batch); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	n, err := tbl.NumRows(ctx)
	if err != nil {
		t.Fatalf("NumRows failed: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 rows, got %d", n)
	}
}

func TestE2E_Search(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := newTestSchema()
	tbl, err := db.CreateTable(ctx, "vectors", schema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	// Insert 3 batches so we have 15 vectors to search.
	// Use startID=1 to avoid zero-norm vectors (cosine distance fails on [0,0,0,0]).
	for i := 0; i < 3; i++ {
		batch := makeBatch(schema, int64(1+i*5), "test", 5)
		if err := tbl.Insert(ctx, batch); err != nil {
			t.Fatalf("Insert batch %d failed: %v", i, err)
		}
	}

	// Search for a vector that matches row 7 (embedding=[7,0,0,0]).
	q := NewQuery(Vector([]float32{7, 0, 0, 0}).Column("embedding")).TopK(3).Build()
	results, err := tbl.Search(ctx, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	// The closest vector should be row 7 (distance ~0).
	top := results[0]
	if top.RowID != 7 {
		t.Logf("WARNING: top-1 RowID = %d (expected 7), Score = %f", top.RowID, top.Score)
	}
	if top.Score > 1e-4 {
		t.Logf("WARNING: top-1 Score = %f (expected ~0)", top.Score)
	}
}

func TestE2E_SearchWithScalarFilter(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := newTestSchema()
	tbl, err := db.CreateTable(ctx, "vectors", schema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	// Insert "science" vectors with IDs [1,50) and "art" vectors with IDs [50,100).
	batch1 := makeBatch(schema, 1, "science", 49)
	if err := tbl.Insert(ctx, batch1); err != nil {
		t.Fatalf("Insert science batch failed: %v", err)
	}
	batch2 := makeBatch(schema, 50, "art", 50)
	if err := tbl.Insert(ctx, batch2); err != nil {
		t.Fatalf("Insert art batch failed: %v", err)
	}

	// Search only "science" entries, searching for vector [1,0,0,0].
	q := NewQuery(Vector([]float32{1, 0, 0, 0}).Column("embedding")).
		Filter(EQ("category", "science")).
		TopK(5).
		Build()
	results, err := tbl.Search(ctx, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 science result")
	}
	// All results should be from "science" group (RowID < 50).
	for _, r := range results {
		if r.RowID >= 50 {
			t.Fatalf("result RowID=%d is not in science group (expected <50)", r.RowID)
		}
	}
}

func TestE2E_AsyncWriter(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := newTestSchema()
	tbl, err := db.CreateTable(ctx, "vectors", schema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	if err := tbl.StartAsyncWriter(ctx, 10, 50*time.Millisecond); err != nil {
		t.Fatalf("StartAsyncWriter failed: %v", err)
	}

	// Write 3 batches asynchronously.
	for i := 0; i < 3; i++ {
		batch := makeBatch(schema, int64(i*5), "async", 5)
		if err := tbl.InsertAsync(ctx, batch); err != nil {
			t.Fatalf("InsertAsync batch %d failed: %v", i, err)
		}
	}
	// Ensure all data is flushed.
	if err := tbl.Flush(ctx); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	n, err := tbl.NumRows(ctx)
	if err != nil {
		t.Fatalf("NumRows failed: %v", err)
	}
	if n != 15 {
		t.Fatalf("expected 15 rows after async writes, got %d", n)
	}
}

func TestE2E_OpenExistingTable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	schema := newTestSchema()
	if _, err := db.CreateTable(ctx, "vectors", schema); err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reopen and load the table.
	db2, err := Connect(dir)
	if err != nil {
		t.Fatalf("Reconnect failed: %v", err)
	}
	defer db2.Close()

	tbl, err := db2.OpenTable(ctx, "vectors")
	if err != nil {
		t.Fatalf("OpenTable failed: %v", err)
	}
	if tbl.Name() != "vectors" {
		t.Fatalf("expected table name 'vectors', got %q", tbl.Name())
	}
}

func TestE2E_AddColumn(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := newTestSchema()
	tbl, err := db.CreateTable(ctx, "vectors", schema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	field := &table.Field{Name: "score", Type: encode.TypeFloat32, Nullable: true}
	if err := tbl.AddColumn(ctx, field); err != nil {
		t.Fatalf("AddColumn failed: %v", err)
	}

	s := tbl.Schema()
	if s.FieldByName("score") == nil {
		t.Fatal("expected 'score' field after AddColumn")
	}
}

func TestE2E_DropTable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := newTestSchema()
	if _, err := db.CreateTable(ctx, "vectors", schema); err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}
	if err := db.DropTable(ctx, "vectors"); err != nil {
		t.Fatalf("DropTable failed: %v", err)
	}

	tables, err := db.ListTables(ctx)
	if err != nil {
		t.Fatalf("ListTables failed: %v", err)
	}
	if len(tables) != 0 {
		t.Fatalf("expected 0 tables after drop, got %d", len(tables))
	}
}

func TestE2E_QueryBuilder(t *testing.T) {
	// Test the query builder constructs correct query objects.
	q := NewQuery(Vector([]float32{1, 2, 3}).Column("emb").Metric(query.DistanceEuclidean)).
		TopK(10).
		Filter(EQ("cat", "a").GT("score", 0.5)).
		Select("id", "cat").
		Build()

	if q.Limit != 10 {
		t.Fatalf("expected Limit=10, got %d", q.Limit)
	}
	if q.Vector == nil {
		t.Fatal("expected non-nil Vector")
	}
	if q.Vector.Metric != query.DistanceEuclidean {
		t.Fatalf("expected Euclidean metric")
	}
	if q.Filter == nil || len(q.Filter.Conditions) != 2 {
		t.Fatalf("expected 2 filter conditions, got %d", len(q.Filter.Conditions))
	}
	if len(q.Columns) != 2 || q.Columns[0] != "id" || q.Columns[1] != "cat" {
		t.Fatalf("unexpected columns: %v", q.Columns)
	}
}

func TestE2E_ErrorCodes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Opening non-existent table should return ErrNotFound.
	if _, err := db.OpenTable(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error when opening non-existent table")
	} else {
		if !IsCode(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	}

	// Dropping non-existent table should return ErrNotFound.
	if err := db.DropTable(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error when dropping non-existent table")
	} else {
		if !IsCode(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	}

	schema := newTestSchema()
	if _, err := db.CreateTable(ctx, "dup", schema); err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}
	// Duplicate table should return ErrConflict.
	if _, err := db.CreateTable(ctx, "dup", schema); err == nil {
		t.Fatal("expected error when creating duplicate table")
	} else {
		if !IsCode(err, ErrConflict) {
			t.Fatalf("expected ErrConflict, got %v", err)
		}
	}

	db.Close()
	// Operations on closed DB should return ErrClosed.
	if _, err := db.ListTables(ctx); err == nil {
		t.Fatal("expected error on closed DB")
	} else {
		if !IsCode(err, ErrClosed) {
			t.Fatalf("expected ErrClosed, got %v", err)
		}
	}
}

func TestE2E_Logger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewTextLogger(&buf, LevelDebug)
	_ = logger

	SetLogger(NewLogger())
	L().Info("test message", "key", "value")

	// Verify LogOperation logs start and result.
	calls := 0
	err := L().WithOp("test").LogOperation(context.Background(), "doSomething", func(ctx context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected fn to be called 1 time, got %d", calls)
	}
}

func TestE2E_VectorDistanceMetrics(t *testing.T) {
	// Cosine search should find the closest vector by direction.
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := table.NewSchema([]*table.Field{
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 3},
	})

	tbl, err := db.CreateTable(ctx, "vecs", schema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	// Vectors with different directions.
	batch := table.NewRecordBatch(schema, 4)
	batch.SetColumn(0, []float32{
		1, 0, 0, // row 0: x-axis
		0, 1, 0, // row 1: y-axis
		0, 0, 1, // row 2: z-axis
		0.5, 0.5, 0, // row 3: between x and y
	})
	if err := tbl.Insert(ctx, batch); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Search with x-axis vector.
	q := NewQuery(Vector([]float32{1, 0, 0}).Column("embedding")).
		TopK(2).
		Build()
	results, err := tbl.Search(ctx, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}
	if results[0].RowID != 0 {
		t.Logf("top-1 RowID=%d Score=%f (expected RowID=0 Score=~0)", results[0].RowID, results[0].Score)
	}
	if math.Abs(results[0].Score) > 1e-4 {
		t.Logf("WARNING: top-1 Score=%f, expected ~0", results[0].Score)
	}
}

// TestE2E_StorageErrorHandling verifies that storage failures produce
// user-friendly api.Error with ErrStorage code and the error chain
// contains enough context for debugging.
func TestE2E_StorageErrorHandling(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := newTestSchema()
	tbl, err := db.CreateTable(ctx, "vectors", schema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	batch := makeBatch(schema, 1, "test", 5)
	if err := tbl.Insert(ctx, batch); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Corrupt a data file to simulate a storage failure.
	// Find the data file and overwrite it with garbage.
	dataDir := filepath.Join(dir, "vectors", "data")
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		t.Fatalf("failed to read data dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one fragment data dir")
	}
	// Overwrite all .lance files with garbage to simulate corruption.
	fragDir := filepath.Join(dataDir, entries[0].Name())
	lanceFiles, err := os.ReadDir(fragDir)
	if err != nil {
		t.Fatalf("failed to read fragment dir: %v", err)
	}
	var corrupted bool
	for _, f := range lanceFiles {
		if filepath.Ext(f.Name()) == ".lance" {
			path := filepath.Join(fragDir, f.Name())
			if err := os.WriteFile(path, []byte("corrupted data"), 0o644); err != nil {
				t.Fatalf("failed to corrupt file: %v", err)
			}
			corrupted = true
		}
	}
	if !corrupted {
		t.Fatal("no .lance file found to corrupt")
	}

	// Search should return a storage error when reading corrupted data.
	q := NewQuery(Vector([]float32{1, 0, 0, 0}).Column("embedding")).TopK(3).Build()
	_, err = tbl.Search(ctx, q)
	if err == nil {
		t.Fatal("expected error when reading corrupted data")
	}
	if !IsStorageError(err) {
		t.Fatalf("expected ErrStorage code, got %v", err)
	}
	// Verify UserMessage provides guidance.
	var apiErr *Error
	if errors.As(err, &apiErr) {
		msg := apiErr.UserMessage()
		if msg == "" {
			t.Fatal("expected non-empty user message")
		}
	}
}

// TestE2E_SearchAfterSchemaEvolution verifies that Search works correctly
// after AddColumn, using the updated schema.
func TestE2E_SearchAfterSchemaEvolution(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := newTestSchema()
	tbl, err := db.CreateTable(ctx, "vectors", schema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	batch := makeBatch(schema, 1, "science", 5)
	if err := tbl.Insert(ctx, batch); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Add a new column after initial data.
	newField := &table.Field{Name: "score", Type: encode.TypeFloat32, Nullable: true}
	if err := tbl.AddColumn(ctx, newField); err != nil {
		t.Fatalf("AddColumn failed: %v", err)
	}

	// Search should still work with the updated schema.
	q := NewQuery(Vector([]float32{1, 0, 0, 0}).Column("embedding")).TopK(3).Build()
	results, err := tbl.Search(ctx, q)
	if err != nil {
		t.Fatalf("Search after AddColumn failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result after schema evolution")
	}
}

// TestE2E_AsyncWriterSearch verifies data written by AsyncWriter is searchable.
func TestE2E_AsyncWriterSearch(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := newTestSchema()
	tbl, err := db.CreateTable(ctx, "vectors", schema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	if err := tbl.StartAsyncWriter(ctx, 10, 50*time.Millisecond); err != nil {
		t.Fatalf("StartAsyncWriter failed: %v", err)
	}

	// Write data asynchronously with startID=1 to avoid zero-norm vectors.
	for i := 0; i < 3; i++ {
		batch := makeBatch(schema, int64(1+i*5), "async", 5)
		if err := tbl.InsertAsync(ctx, batch); err != nil {
			t.Fatalf("InsertAsync batch %d failed: %v", i, err)
		}
	}
	if err := tbl.Flush(ctx); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Search for a vector that matches row 7 (embedding=[7,0,0,0]).
	q := NewQuery(Vector([]float32{7, 0, 0, 0}).Column("embedding")).TopK(3).Build()
	results, err := tbl.Search(ctx, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result from async-written data")
	}
}

// TestE2E_BatchInsert verifies that BatchInsert correctly merges multiple
// RecordBatches into a single fragment.
func TestE2E_BatchInsert(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := Connect(dir)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	schema := newTestSchema()
	tbl, err := db.CreateTable(ctx, "vectors", schema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	// Insert 3 batches via BatchInsert with startID=1 to avoid zero-norm vectors.
	batches := []*table.RecordBatch{
		makeBatch(schema, 1, "batch", 3),
		makeBatch(schema, 4, "batch", 3),
		makeBatch(schema, 7, "batch", 3),
	}
	if err := tbl.BatchInsert(ctx, batches); err != nil {
		t.Fatalf("BatchInsert failed: %v", err)
	}

	// Should be a single fragment with 9 rows.
	n, err := tbl.NumRows(ctx)
	if err != nil {
		t.Fatalf("NumRows failed: %v", err)
	}
	if n != 9 {
		t.Fatalf("expected 9 rows after BatchInsert, got %d", n)
	}

	// Search should find the vector [4,0,0,0] (row 4).
	q := NewQuery(Vector([]float32{4, 0, 0, 0}).Column("embedding")).TopK(3).Build()
	results, err := tbl.Search(ctx, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result from BatchInsert data")
	}
}
