package table

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/storage"
)

func newTestWriter(t *testing.T) (*FragmentWriter, storage.Store) {
	t.Helper()
	dir := t.TempDir()
	store := storage.NewLocalFS(dir)
	schema := NewSchema([]*Field{
		{Name: "id", Type: encode.TypeInt64},
		{Name: "score", Type: encode.TypeFloat32},
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 128},
	})
	return NewFragmentWriter(store, schema, 0, "tbl", encode.CompressionNone), store
}

func TestFragmentWriterWriteColumnInt64(t *testing.T) {
	ctx := context.Background()
	w, store := newTestWriter(t)

	data := []int64{1, 2, 3, 4, 5}
	if err := w.WriteColumn(ctx, 0, data); err != nil {
		t.Fatalf("WriteColumn failed: %v", err)
	}

	// Verify file was written.
	exists, err := store.Exists(ctx, filepath.Join("tbl/data/f0/col_0.lance"))
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected col_0.lance to exist")
	}

	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	if frag.NumDataFiles() != 1 {
		t.Errorf("expected 1 data file, got %d", frag.NumDataFiles())
	}
	if frag.NumRows != 5 {
		t.Errorf("expected 5 rows, got %d", frag.NumRows)
	}
	df := frag.DataFiles[0]
	if df.ColumnID != 0 {
		t.Errorf("expected column ID 0, got %d", df.ColumnID)
	}
	if df.NumRows != 5 {
		t.Errorf("expected data file numRows 5, got %d", df.NumRows)
	}
	if df.FileSize <= 0 {
		t.Errorf("expected positive file size, got %d", df.FileSize)
	}
}

func TestFragmentWriterWriteColumnVector(t *testing.T) {
	ctx := context.Background()
	w, _ := newTestWriter(t)

	dim := 128
	rows := int64(10)
	vec := make([]float32, int(rows)*dim)
	for i := range vec {
		vec[i] = float32(i)
	}

	if err := w.WriteColumn(ctx, 2, vec); err != nil {
		t.Fatalf("WriteColumn failed: %v", err)
	}

	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	if frag.NumRows != rows {
		t.Errorf("expected %d rows, got %d", rows, frag.NumRows)
	}
	if frag.DataFiles[0].NumRows != rows {
		t.Errorf("expected data file numRows %d, got %d", rows, frag.DataFiles[0].NumRows)
	}
}

func TestFragmentWriterWriteBatch(t *testing.T) {
	ctx := context.Background()
	w, _ := newTestWriter(t)

	numRows := int64(4)
	batch := NewRecordBatch(w.schema, numRows)
	batch.SetColumn(0, []int64{10, 20, 30, 40})
	batch.SetColumn(1, []float32{1.5, 2.5, 3.5, 4.5})

	if err := w.WriteBatch(ctx, batch); err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}

	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	if frag.NumDataFiles() != 2 {
		t.Errorf("expected 2 data files, got %d", frag.NumDataFiles())
	}
	if frag.NumRows != numRows {
		t.Errorf("expected %d rows, got %d", numRows, frag.NumRows)
	}
}

func TestFragmentWriterFinishReturnsFragment(t *testing.T) {
	ctx := context.Background()
	w, _ := newTestWriter(t)

	if err := w.WriteColumn(ctx, 0, []int64{1, 2}); err != nil {
		t.Fatalf("WriteColumn failed: %v", err)
	}
	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	if frag.ID != 0 {
		t.Errorf("expected fragment ID 0, got %d", frag.ID)
	}
	if frag.NumRows != 2 {
		t.Errorf("expected 2 rows, got %d", frag.NumRows)
	}
	if frag.PhysicalRows != 2 {
		t.Errorf("expected 2 physical rows, got %d", frag.PhysicalRows)
	}
}

func TestFragmentWriterWriteColumnUnknownID(t *testing.T) {
	ctx := context.Background()
	w, _ := newTestWriter(t)

	if err := w.WriteColumn(ctx, 99, []int64{1}); err == nil {
		t.Error("expected error for unknown column ID")
	}
}

func TestFragmentWriterWriteColumnTypeMismatch(t *testing.T) {
	ctx := context.Background()
	w, _ := newTestWriter(t)

	if err := w.WriteColumn(ctx, 0, []string{"not int64"}); err == nil {
		t.Error("expected error for type mismatch")
	}
}

func TestFragmentWriterWriteColumnEmptyData(t *testing.T) {
	ctx := context.Background()
	w, _ := newTestWriter(t)

	if err := w.WriteColumn(ctx, 0, []int64{}); err != nil {
		t.Fatalf("WriteColumn with empty data failed: %v", err)
	}
	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	if frag.NumRows != 0 {
		t.Errorf("expected 0 rows, got %d", frag.NumRows)
	}
}

func TestRecordBatch(t *testing.T) {
	schema := NewSchema([]*Field{{Name: "id", Type: encode.TypeInt64}})
	batch := NewRecordBatch(schema, 3)
	if batch.NumRows != 3 {
		t.Errorf("expected 3 rows, got %d", batch.NumRows)
	}
	batch.SetColumn(0, []int64{1, 2, 3})
	if got := batch.Column(0); got == nil {
		t.Error("expected column data")
	}
	if got := batch.Column(99); got != nil {
		t.Error("expected nil for unset column")
	}
}
