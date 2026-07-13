package table

import (
	"context"
	"reflect"
	"testing"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/storage"
)

func newReaderTestEnv(t *testing.T) (*FragmentWriter, *FragmentReader, storage.ObjectStore) {
	t.Helper()
	dir := t.TempDir()
	store := storage.NewLocalFS(dir)
	schema := NewSchema([]*Field{
		{Name: "id", Type: encode.TypeInt64},
		{Name: "score", Type: encode.TypeFloat32},
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 8},
	})
	w := NewFragmentWriter(store, schema, 0, "tbl", encode.CompressionNone)
	r := NewFragmentReader(store, schema, encode.CompressionNone)
	return w, r, store
}

func TestFragmentReaderReadColumnInt64(t *testing.T) {
	ctx := context.Background()
	w, r, _ := newReaderTestEnv(t)

	want := []int64{1, 2, 3, 4, 5}
	if err := w.WriteColumn(ctx, 0, want); err != nil {
		t.Fatalf("WriteColumn failed: %v", err)
	}
	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	got, err := r.ReadColumn(ctx, frag, 0)
	if err != nil {
		t.Fatalf("ReadColumn failed: %v", err)
	}
	gotSlice, ok := got.([]int64)
	if !ok {
		t.Fatalf("expected []int64, got %T", got)
	}
	if !reflect.DeepEqual(gotSlice, want) {
		t.Errorf("expected %v, got %v", want, gotSlice)
	}
}

func TestFragmentReaderReadColumnVector(t *testing.T) {
	ctx := context.Background()
	w, r, _ := newReaderTestEnv(t)

	dim := 8
	rows := int64(5)
	want := make([]float32, int(rows)*dim)
	for i := range want {
		want[i] = float32(i) * 0.5
	}
	if err := w.WriteColumn(ctx, 2, want); err != nil {
		t.Fatalf("WriteColumn failed: %v", err)
	}
	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	got, err := r.ReadColumn(ctx, frag, 2)
	if err != nil {
		t.Fatalf("ReadColumn failed: %v", err)
	}
	gotSlice, ok := got.([]float32)
	if !ok {
		t.Fatalf("expected []float32, got %T", got)
	}
	if len(gotSlice) != len(want) {
		t.Fatalf("length mismatch: expected %d, got %d", len(want), len(gotSlice))
	}
	if !reflect.DeepEqual(gotSlice, want) {
		t.Errorf("vector data mismatch")
	}
}

func TestFragmentReaderReadBatch(t *testing.T) {
	ctx := context.Background()
	w, r, _ := newReaderTestEnv(t)

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

	got, err := r.ReadBatch(ctx, frag, []int32{0, 1})
	if err != nil {
		t.Fatalf("ReadBatch failed: %v", err)
	}
	if got.NumRows != numRows {
		t.Errorf("expected %d rows, got %d", numRows, got.NumRows)
	}

	idCol, ok := got.Column(0).([]int64)
	if !ok {
		t.Fatalf("expected []int64 for column 0, got %T", got.Column(0))
	}
	wantID := []int64{10, 20, 30, 40}
	if !reflect.DeepEqual(idCol, wantID) {
		t.Errorf("column 0: expected %v, got %v", wantID, idCol)
	}

	scoreCol, ok := got.Column(1).([]float32)
	if !ok {
		t.Fatalf("expected []float32 for column 1, got %T", got.Column(1))
	}
	wantScore := []float32{1.5, 2.5, 3.5, 4.5}
	if !reflect.DeepEqual(scoreCol, wantScore) {
		t.Errorf("column 1: expected %v, got %v", wantScore, scoreCol)
	}
}

func TestFragmentReaderReadColumnNonExistent(t *testing.T) {
	ctx := context.Background()
	w, r, _ := newReaderTestEnv(t)

	if err := w.WriteColumn(ctx, 0, []int64{1}); err != nil {
		t.Fatalf("WriteColumn failed: %v", err)
	}
	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	// Column 1 was never written.
	if _, err := r.ReadColumn(ctx, frag, 1); err == nil {
		t.Error("expected error for non-existent column in fragment")
	}
}

func TestFragmentReaderReadColumnUnknownSchemaID(t *testing.T) {
	ctx := context.Background()
	w, r, _ := newReaderTestEnv(t)

	if err := w.WriteColumn(ctx, 0, []int64{1}); err != nil {
		t.Fatalf("WriteColumn failed: %v", err)
	}
	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	if _, err := r.ReadColumn(ctx, frag, 99); err == nil {
		t.Error("expected error for unknown schema ID")
	}
}

func TestFragmentReaderReadBatchEmpty(t *testing.T) {
	ctx := context.Background()
	w, r, _ := newReaderTestEnv(t)

	if err := w.WriteColumn(ctx, 0, []int64{1, 2}); err != nil {
		t.Fatalf("WriteColumn failed: %v", err)
	}
	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	got, err := r.ReadBatch(ctx, frag, nil)
	if err != nil {
		t.Fatalf("ReadBatch with empty column list failed: %v", err)
	}
	if got.NumRows != 2 {
		t.Errorf("expected 2 rows, got %d", got.NumRows)
	}
	if len(got.Columns) != 0 {
		t.Errorf("expected 0 columns, got %d", len(got.Columns))
	}
}

func TestFragmentReaderRoundtripMultipleColumns(t *testing.T) {
	ctx := context.Background()
	w, r, _ := newReaderTestEnv(t)

	numRows := int64(3)
	batch := NewRecordBatch(w.schema, numRows)
	idData := []int64{1, 2, 3}
	scoreData := []float32{0.1, 0.2, 0.3}
	embedData := make([]float32, int(numRows)*8)
	for i := range embedData {
		embedData[i] = float32(i)
	}
	batch.SetColumn(0, idData)
	batch.SetColumn(1, scoreData)
	batch.SetColumn(2, embedData)
	if err := w.WriteBatch(ctx, batch); err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}
	frag, err := w.Finish()
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	got, err := r.ReadBatch(ctx, frag, []int32{0, 1, 2})
	if err != nil {
		t.Fatalf("ReadBatch failed: %v", err)
	}
	if !reflect.DeepEqual(got.Column(0).([]int64), idData) {
		t.Errorf("id column mismatch")
	}
	if !reflect.DeepEqual(got.Column(1).([]float32), scoreData) {
		t.Errorf("score column mismatch")
	}
	if !reflect.DeepEqual(got.Column(2).([]float32), embedData) {
		t.Errorf("embedding column mismatch")
	}
}
