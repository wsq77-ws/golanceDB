package table

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/storage"
)

func newTestTableSchema() *Schema {
	return NewSchema([]*Field{
		{Name: "id", Type: encode.TypeInt64},
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 4},
	})
}

func newTestTable(t *testing.T) (*Table, storage.Store) {
	t.Helper()
	dir := t.TempDir()
	store := storage.NewLocalFS("")
	schema := newTestTableSchema()
	ctx := context.Background()
	tbl, err := Create(ctx, "test_table", filepath.Join(dir, "tbl"), schema, store, encode.CompressionNone)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	return tbl, store
}

func newTestBatch(schema *Schema, n int) *RecordBatch {
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = int64(i)
	}
	embeddings := make([]float32, n*4)
	for i := range embeddings {
		embeddings[i] = float32(i) * 0.1
	}
	batch := NewRecordBatch(schema, int64(n))
	batch.SetColumn(0, ids)
	batch.SetColumn(1, embeddings)
	return batch
}

func TestTableCreateOpenRoundtrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := storage.NewLocalFS("")
	schema := newTestTableSchema()

	tbl, err := Create(ctx, "mytable", filepath.Join(dir, "tbl"), schema, store, encode.CompressionNone)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := tbl.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	tbl2, err := Open(ctx, "mytable", filepath.Join(dir, "tbl"), store, encode.CompressionNone)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	s := tbl2.Schema()
	if s.NumFields() != 2 {
		t.Errorf("expected 2 fields, got %d", s.NumFields())
	}
	if !s.HasField("id") || !s.HasField("embedding") {
		t.Error("expected id and embedding fields")
	}

	if tbl2.Name() != "mytable" {
		t.Errorf("expected name mytable, got %s", tbl2.Name())
	}

	m, err := tbl2.LatestManifest(ctx)
	if err != nil {
		t.Fatalf("LatestManifest failed: %v", err)
	}
	if m.Version != 1 {
		t.Errorf("expected version 1, got %d", m.Version)
	}
	if len(m.Fragments) != 0 {
		t.Errorf("expected 0 fragments, got %d", len(m.Fragments))
	}
}

func TestTableInsert(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)
	schema := tbl.Schema()

	batch := newTestBatch(schema, 10)
	frag, err := tbl.Insert(ctx, batch)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if frag.ID != 0 {
		t.Errorf("expected fragment ID 0, got %d", frag.ID)
	}
	if frag.NumRows != 10 {
		t.Errorf("expected 10 rows, got %d", frag.NumRows)
	}

	m, err := tbl.LatestManifest(ctx)
	if err != nil {
		t.Fatalf("LatestManifest failed: %v", err)
	}
	if m.Version != 2 {
		t.Errorf("expected version 2, got %d", m.Version)
	}
	if len(m.Fragments) != 1 {
		t.Errorf("expected 1 fragment, got %d", len(m.Fragments))
	}
	if m.MaxFragmentID != 0 {
		t.Errorf("expected MaxFragmentID 0, got %d", m.MaxFragmentID)
	}
}

func TestTableInsertMultiple(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)
	schema := tbl.Schema()

	for i := 0; i < 3; i++ {
		batch := newTestBatch(schema, 5)
		frag, err := tbl.Insert(ctx, batch)
		if err != nil {
			t.Fatalf("Insert %d failed: %v", i, err)
		}
		if frag.ID != int32(i) {
			t.Errorf("expected fragment ID %d, got %d", i, frag.ID)
		}
	}

	m, err := tbl.LatestManifest(ctx)
	if err != nil {
		t.Fatalf("LatestManifest failed: %v", err)
	}
	if m.Version != 4 {
		t.Errorf("expected version 4, got %d", m.Version)
	}
	if len(m.Fragments) != 3 {
		t.Errorf("expected 3 fragments, got %d", len(m.Fragments))
	}
	if m.MaxFragmentID != 2 {
		t.Errorf("expected MaxFragmentID 2, got %d", m.MaxFragmentID)
	}
}

func TestTableLatestManifest(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)
	schema := tbl.Schema()

	m, err := tbl.LatestManifest(ctx)
	if err != nil {
		t.Fatalf("LatestManifest failed: %v", err)
	}
	if m.Version != 1 {
		t.Errorf("expected version 1, got %d", m.Version)
	}

	batch := newTestBatch(schema, 5)
	if _, err := tbl.Insert(ctx, batch); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	m, err = tbl.LatestManifest(ctx)
	if err != nil {
		t.Fatalf("LatestManifest failed: %v", err)
	}
	if m.Version != 2 {
		t.Errorf("expected version 2, got %d", m.Version)
	}
	if len(m.Fragments) != 1 {
		t.Errorf("expected 1 fragment, got %d", len(m.Fragments))
	}
}

func TestTableCheckoutVersion(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)
	schema := tbl.Schema()

	batch := newTestBatch(schema, 5)
	if _, err := tbl.Insert(ctx, batch); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	m1, err := tbl.CheckoutVersion(ctx, 1)
	if err != nil {
		t.Fatalf("CheckoutVersion(1) failed: %v", err)
	}
	if m1.Version != 1 {
		t.Errorf("expected version 1, got %d", m1.Version)
	}
	if len(m1.Fragments) != 0 {
		t.Errorf("expected 0 fragments, got %d", len(m1.Fragments))
	}

	m2, err := tbl.CheckoutVersion(ctx, 2)
	if err != nil {
		t.Fatalf("CheckoutVersion(2) failed: %v", err)
	}
	if m2.Version != 2 {
		t.Errorf("expected version 2, got %d", m2.Version)
	}
	if len(m2.Fragments) != 1 {
		t.Errorf("expected 1 fragment, got %d", len(m2.Fragments))
	}

	if _, err := tbl.CheckoutVersion(ctx, 999); err == nil {
		t.Error("expected error for non-existent version")
	}
}

func TestTableNumFragmentsAndRows(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)
	schema := tbl.Schema()

	n, err := tbl.NumFragments(ctx)
	if err != nil {
		t.Fatalf("NumFragments failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 fragments, got %d", n)
	}

	rows, err := tbl.NumRows(ctx)
	if err != nil {
		t.Fatalf("NumRows failed: %v", err)
	}
	if rows != 0 {
		t.Errorf("expected 0 rows, got %d", rows)
	}

	if _, err := tbl.Insert(ctx, newTestBatch(schema, 10)); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	n, _ = tbl.NumFragments(ctx)
	if n != 1 {
		t.Errorf("expected 1 fragment, got %d", n)
	}
	rows, _ = tbl.NumRows(ctx)
	if rows != 10 {
		t.Errorf("expected 10 rows, got %d", rows)
	}

	if _, err := tbl.Insert(ctx, newTestBatch(schema, 5)); err != nil {
		t.Fatalf("Insert 2 failed: %v", err)
	}

	n, _ = tbl.NumFragments(ctx)
	if n != 2 {
		t.Errorf("expected 2 fragments, got %d", n)
	}
	rows, _ = tbl.NumRows(ctx)
	if rows != 15 {
		t.Errorf("expected 15 rows, got %d", rows)
	}
}

func TestTableListVersions(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)
	schema := tbl.Schema()

	versions, err := tbl.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 || versions[0] != 1 {
		t.Errorf("expected [1], got %v", versions)
	}

	if _, err := tbl.Insert(ctx, newTestBatch(schema, 5)); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	versions, _ = tbl.ListVersions(ctx)
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
	want := []int64{1, 2}
	for i, v := range want {
		if versions[i] != v {
			t.Errorf("position %d: expected %d, got %d", i, v, versions[i])
		}
	}
}

func TestTableAddColumn(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)

	initialVersions, _ := tbl.ListVersions(ctx)

	if err := tbl.AddColumn(ctx, &Field{Name: "score", Type: encode.TypeFloat32}); err != nil {
		t.Fatalf("AddColumn failed: %v", err)
	}

	schema := tbl.Schema()
	if schema.NumFields() != 3 {
		t.Errorf("expected 3 fields, got %d", schema.NumFields())
	}
	scoreField := schema.FieldByName("score")
	if scoreField == nil {
		t.Fatal("expected score field")
	}
	if scoreField.ID != 2 {
		t.Errorf("expected score field ID 2, got %d", scoreField.ID)
	}

	afterVersions, _ := tbl.ListVersions(ctx)
	if len(afterVersions) != len(initialVersions)+1 {
		t.Errorf("expected %d versions, got %d", len(initialVersions)+1, len(afterVersions))
	}

	if err := tbl.AddColumn(ctx, &Field{Name: "score", Type: encode.TypeFloat32}); err == nil {
		t.Error("expected error for duplicate column")
	}
}

func TestTableDropColumn(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)

	initialVersions, _ := tbl.ListVersions(ctx)

	if err := tbl.DropColumn(ctx, "embedding"); err != nil {
		t.Fatalf("DropColumn failed: %v", err)
	}

	schema := tbl.Schema()
	if schema.NumFields() != 1 {
		t.Errorf("expected 1 field, got %d", schema.NumFields())
	}
	if schema.HasField("embedding") {
		t.Error("expected embedding field to be removed")
	}

	afterVersions, _ := tbl.ListVersions(ctx)
	if len(afterVersions) != len(initialVersions)+1 {
		t.Errorf("expected %d versions, got %d", len(initialVersions)+1, len(afterVersions))
	}

	if err := tbl.DropColumn(ctx, "nonexistent"); err == nil {
		t.Error("expected error for non-existent column")
	}
}

func TestTableConcurrentReadsAndWrites(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := storage.NewLocalFS("")
	schema := newTestTableSchema()
	tbl, err := Create(ctx, "test_table", filepath.Join(dir, "tbl"), schema, store, encode.CompressionNone)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(stop)
		for i := 0; i < 20; i++ {
			batch := newTestBatch(schema, 5)
			if _, err := tbl.Insert(ctx, batch); err != nil {
				t.Errorf("Insert failed: %v", err)
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				if _, err := tbl.LatestManifest(ctx); err != nil {
					t.Errorf("LatestManifest failed: %v", err)
					return
				}
				_ = tbl.Schema()
				_ = tbl.Name()
			}
		}
	}()

	wg.Wait()

	rows, err := tbl.NumRows(ctx)
	if err != nil {
		t.Fatalf("NumRows failed: %v", err)
	}
	if rows != 100 {
		t.Errorf("expected 100 rows, got %d", rows)
	}
}

func TestTableSchemaNameConcurrent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := storage.NewLocalFS("")
	schema := newTestTableSchema()
	tbl, err := Create(ctx, "test_table", filepath.Join(dir, "tbl"), schema, store, encode.CompressionNone)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var wg sync.WaitGroup

	// Concurrent reads: 3 goroutines reading Schema() and Name() 100 times each.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = tbl.Schema()
				_ = tbl.Name()
			}
		}()
	}
	wg.Wait()
}

func TestTableOpenNonExistent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := storage.NewLocalFS("")

	if _, err := Open(ctx, "missing", filepath.Join(dir, "tbl"), store, encode.CompressionNone); err == nil {
		t.Error("expected error opening non-existent table")
	}
}

func TestTableAddColumnAfterInsert(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)
	schema := tbl.Schema()

	if _, err := tbl.Insert(ctx, newTestBatch(schema, 5)); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	if err := tbl.AddColumn(ctx, &Field{Name: "label", Type: encode.TypeString}); err != nil {
		t.Fatalf("AddColumn failed: %v", err)
	}

	m, err := tbl.LatestManifest(ctx)
	if err != nil {
		t.Fatalf("LatestManifest failed: %v", err)
	}
	if len(m.Fragments) != 1 {
		t.Errorf("expected 1 fragment preserved, got %d", len(m.Fragments))
	}
	if m.Schema.NumFields() != 3 {
		t.Errorf("expected 3 fields, got %d", m.Schema.NumFields())
	}
}

func TestTableCheckoutVersionAfterEviction(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)
	schema := tbl.Schema()

	for i := 0; i < 15; i++ {
		if _, err := tbl.Insert(ctx, newTestBatch(schema, 2)); err != nil {
			t.Fatalf("Insert %d failed: %v", i, err)
		}
	}

	m, err := tbl.CheckoutVersion(ctx, 1)
	if err != nil {
		t.Fatalf("CheckoutVersion(1) failed after eviction: %v", err)
	}
	if m.Version != 1 {
		t.Errorf("expected version 1, got %d", m.Version)
	}
	if len(m.Fragments) != 0 {
		t.Errorf("expected 0 fragments, got %d", len(m.Fragments))
	}

	latest, err := tbl.LatestManifest(ctx)
	if err != nil {
		t.Fatalf("LatestManifest failed: %v", err)
	}
	if latest.Version != 16 {
		t.Errorf("expected version 16, got %d", latest.Version)
	}
	if len(latest.Fragments) != 15 {
		t.Errorf("expected 15 fragments, got %d", len(latest.Fragments))
	}
}

func TestTableInsertErrorOnUnknownColumn(t *testing.T) {
	ctx := context.Background()
	tbl, _ := newTestTable(t)

	batch := NewRecordBatch(tbl.Schema(), 3)
	batch.SetColumn(99, []int64{1, 2, 3})

	_, err := tbl.Insert(ctx, batch)
	if err == nil {
		t.Error("expected error for unknown column ID")
	}
}
