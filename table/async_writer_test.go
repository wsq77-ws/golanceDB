package table

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/storage"
)

func newAsyncTestSetup(t *testing.T, maxBatchSize int, flushInterval time.Duration) (*AsyncWriter, *ManifestStore, storage.Store, *Schema) {
	t.Helper()
	dir := t.TempDir()
	store := storage.NewLocalFS(dir)
	ms := NewManifestStore(store, "tbl/_versions")
	schema := NewSchema([]*Field{
		{Name: "id", Type: encode.TypeInt64},
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 4},
	})
	w := NewAsyncWriter(store, ms, schema, "tbl", encode.CompressionNone, maxBatchSize, flushInterval)
	return w, ms, store, schema
}

func makeTestBatch(schema *Schema, id int64) *RecordBatch {
	batch := NewRecordBatch(schema, 1)
	batch.SetColumn(0, []int64{id})
	batch.SetColumn(1, []float32{float32(id), float32(id) + 1, float32(id) + 2, float32(id) + 3})
	return batch
}

func TestAsyncWriterBasicWriteFlush(t *testing.T) {
	ctx := context.Background()
	w, ms, _, schema := newAsyncTestSetup(t, 100, 0)
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	batch := makeTestBatch(schema, 42)
	if err := w.Write(ctx, batch); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	versions, err := ms.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 || versions[0] != 1 {
		t.Fatalf("expected [1], got %v", versions)
	}

	manifest, err := ms.Read(ctx, 1)
	if err != nil {
		t.Fatalf("Read manifest failed: %v", err)
	}
	if len(manifest.Fragments) != 1 {
		t.Fatalf("expected 1 fragment, got %d", len(manifest.Fragments))
	}
	frag := manifest.Fragments[0]
	if frag.ID != 0 {
		t.Errorf("expected fragment ID 0, got %d", frag.ID)
	}
	if frag.NumRows != 1 {
		t.Errorf("expected 1 row, got %d", frag.NumRows)
	}
	if frag.NumDataFiles() != 2 {
		t.Errorf("expected 2 data files, got %d", frag.NumDataFiles())
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestAsyncWriterAutoFlushByBatchSize(t *testing.T) {
	ctx := context.Background()
	w, ms, _, schema := newAsyncTestSetup(t, 2, 0)
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := w.Write(ctx, makeTestBatch(schema, 1)); err != nil {
		t.Fatalf("Write 1 failed: %v", err)
	}
	if err := w.Write(ctx, makeTestBatch(schema, 2)); err != nil {
		t.Fatalf("Write 2 failed: %v", err)
	}

	// Wait for auto-flush triggered by maxBatchSize.
	select {
	case r := <-w.Results():
		if r.Error != nil {
			t.Fatalf("auto-flush error: %v", r.Error)
		}
		if r.Fragment == nil || r.Fragment.NumRows != 2 {
			t.Fatalf("unexpected fragment: %+v", r.Fragment)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for auto-flush")
	}

	if err := w.Write(ctx, makeTestBatch(schema, 3)); err != nil {
		t.Fatalf("Write 3 failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	versions, err := ms.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d: %v", len(versions), versions)
	}

	m1, err := ms.Read(ctx, 1)
	if err != nil {
		t.Fatalf("Read v1 failed: %v", err)
	}
	if m1.Fragments[0].NumRows != 2 {
		t.Errorf("expected v1 fragment 2 rows, got %d", m1.Fragments[0].NumRows)
	}

	m2, err := ms.Read(ctx, 2)
	if err != nil {
		t.Fatalf("Read v2 failed: %v", err)
	}
	if len(m2.Fragments) != 2 {
		t.Fatalf("expected 2 fragments in v2, got %d", len(m2.Fragments))
	}
	if m2.Fragments[1].NumRows != 1 {
		t.Errorf("expected v2 fragment 1 (newest) 1 row, got %d", m2.Fragments[1].NumRows)
	}
}

func TestAsyncWriterCloseFlushesPending(t *testing.T) {
	ctx := context.Background()
	w, ms, _, schema := newAsyncTestSetup(t, 100, 0)
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := w.Write(ctx, makeTestBatch(schema, 7)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Write(ctx, makeTestBatch(schema, 8)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	versions, err := ms.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version after close, got %d", len(versions))
	}
	manifest, err := ms.Read(ctx, 1)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if manifest.Fragments[0].NumRows != 2 {
		t.Errorf("expected 2 rows, got %d", manifest.Fragments[0].NumRows)
	}
}

func TestAsyncWriterFlushNoPendingData(t *testing.T) {
	ctx := context.Background()
	w, _, _, _ := newAsyncTestSetup(t, 100, 0)
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Errorf("expected nil for empty flush, got: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestAsyncWriterResultsChannel(t *testing.T) {
	ctx := context.Background()
	w, _, _, schema := newAsyncTestSetup(t, 100, 0)
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := w.Write(ctx, makeTestBatch(schema, 1)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	select {
	case r := <-w.Results():
		if r.Error != nil {
			t.Errorf("unexpected error: %v", r.Error)
		}
		if r.Fragment == nil {
			t.Error("expected non-nil fragment")
		}
		if r.Manifest == nil {
			t.Error("expected non-nil manifest")
		}
		if r.Manifest.Version != 1 {
			t.Errorf("expected version 1, got %d", r.Manifest.Version)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for result")
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestAsyncWriterConcurrentWrites(t *testing.T) {
	ctx := context.Background()
	w, ms, _, schema := newAsyncTestSetup(t, 1000, 0)
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	const numGoroutines = 10
	const writesPerGoroutine = 10
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines*writesPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				id := int64(start*writesPerGoroutine + j + 1)
				if err := w.Write(ctx, makeTestBatch(schema, id)); err != nil {
					errs <- err
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("concurrent Write failed: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	versions, err := ms.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	var totalRows int64
	for _, v := range versions {
		m, err := ms.Read(ctx, v)
		if err != nil {
			t.Fatalf("Read v%d failed: %v", v, err)
		}
		for _, f := range m.Fragments {
			totalRows += f.NumRows
		}
	}

	expected := int64(numGoroutines * writesPerGoroutine)
	if totalRows != expected {
		t.Errorf("expected %d total rows, got %d", expected, totalRows)
	}
}
