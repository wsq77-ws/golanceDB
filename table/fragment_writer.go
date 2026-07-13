package table

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/storage"
)

// RecordBatch holds columnar data for a batch of rows.
type RecordBatch struct {
	Schema  *Schema
	Columns map[int32]interface{}
	NumRows int64
}

// NewRecordBatch creates a RecordBatch with the given schema and row count.
func NewRecordBatch(schema *Schema, numRows int64) *RecordBatch {
	return &RecordBatch{
		Schema:  schema,
		Columns: make(map[int32]interface{}),
		NumRows: numRows,
	}
}

// SetColumn sets the data for a column.
func (b *RecordBatch) SetColumn(columnID int32, data interface{}) {
	b.Columns[columnID] = data
}

// Column returns the data for a column, or nil if not set.
func (b *RecordBatch) Column(columnID int32) interface{} {
	return b.Columns[columnID]
}

// FragmentWriter writes column data into .lance files.
type FragmentWriter struct {
	store       storage.ObjectStore
	schema      *Schema
	fragmentID  int32
	basePath    string
	compression encode.CompressionType
	encoder     encode.ColumnEncoder
	dataFiles   []*DataFile
	numRows     int64
}

// NewFragmentWriter creates a FragmentWriter for the given fragment ID.
func NewFragmentWriter(store storage.ObjectStore, schema *Schema, fragmentID int32, tablePath string, compression encode.CompressionType) *FragmentWriter {
	return &FragmentWriter{
		store:       store,
		schema:      schema,
		fragmentID:  fragmentID,
		basePath:    filepath.Join(tablePath, "data", fmt.Sprintf("f%d", fragmentID)),
		compression: compression,
		encoder:     encode.NewMiniBlockEncoder(compression),
	}
}

// inferNumRows determines the row count from data based on field type.
func inferNumRows(data interface{}, field *Field) (int64, error) {
	switch field.Type {
	case encode.TypeInt8:
		d, ok := data.([]int8)
		if !ok {
			return 0, fmt.Errorf("table: expected []int8 for field %s, got %T", field.Name, data)
		}
		return int64(len(d)), nil
	case encode.TypeInt16:
		d, ok := data.([]int16)
		if !ok {
			return 0, fmt.Errorf("table: expected []int16 for field %s, got %T", field.Name, data)
		}
		return int64(len(d)), nil
	case encode.TypeInt32:
		d, ok := data.([]int32)
		if !ok {
			return 0, fmt.Errorf("table: expected []int32 for field %s, got %T", field.Name, data)
		}
		return int64(len(d)), nil
	case encode.TypeInt64:
		d, ok := data.([]int64)
		if !ok {
			return 0, fmt.Errorf("table: expected []int64 for field %s, got %T", field.Name, data)
		}
		return int64(len(d)), nil
	case encode.TypeFloat32:
		d, ok := data.([]float32)
		if !ok {
			return 0, fmt.Errorf("table: expected []float32 for field %s, got %T", field.Name, data)
		}
		return int64(len(d)), nil
	case encode.TypeFloat64:
		d, ok := data.([]float64)
		if !ok {
			return 0, fmt.Errorf("table: expected []float64 for field %s, got %T", field.Name, data)
		}
		return int64(len(d)), nil
	case encode.TypeString:
		d, ok := data.([]string)
		if !ok {
			return 0, fmt.Errorf("table: expected []string for field %s, got %T", field.Name, data)
		}
		return int64(len(d)), nil
	case encode.TypeBinary:
		d, ok := data.([][]byte)
		if !ok {
			return 0, fmt.Errorf("table: expected [][]byte for field %s, got %T", field.Name, data)
		}
		return int64(len(d)), nil
	case encode.TypeFixedSizeList:
		d, ok := data.([]float32)
		if !ok {
			return 0, fmt.Errorf("table: expected []float32 for field %s, got %T", field.Name, data)
		}
		if field.Dimension <= 0 {
			return 0, fmt.Errorf("table: field %s has invalid dimension %d", field.Name, field.Dimension)
		}
		return int64(len(d)) / int64(field.Dimension), nil
	default:
		return 0, fmt.Errorf("table: unsupported field type %d for field %s", field.Type, field.Name)
	}
}

// WriteColumn encodes and writes a single column to a .lance file.
func (w *FragmentWriter) WriteColumn(ctx context.Context, columnID int32, data interface{}) error {
	field := w.schema.FieldByID(columnID)
	if field == nil {
		return fmt.Errorf("table: field with ID %d not found in schema", columnID)
	}

	encoded, err := w.encoder.Encode(data, field.Type)
	if err != nil {
		return fmt.Errorf("table: encode column %s (id=%d): %w", field.Name, columnID, err)
	}

	path := filepath.Join(w.basePath, fmt.Sprintf("col_%d.lance", columnID))
	if err := w.store.Write(ctx, path, encoded); err != nil {
		return fmt.Errorf("table: write column %s (id=%d) to %q: %w", field.Name, columnID, path, err)
	}

	size, err := w.store.Size(ctx, path)
	if err != nil {
		return fmt.Errorf("table: stat column %s (id=%d) at %q: %w", field.Name, columnID, path, err)
	}

	rowCount, err := inferNumRows(data, field)
	if err != nil {
		return fmt.Errorf("table: infer rows for column %s (id=%d): %w", field.Name, columnID, err)
	}

	w.dataFiles = append(w.dataFiles, &DataFile{
		Path:     path,
		ColumnID: columnID,
		NumRows:  rowCount,
		FileSize: size,
	})

	if rowCount > w.numRows {
		w.numRows = rowCount
	}
	return nil
}

// WriteBatch writes all columns in the batch.
func (w *FragmentWriter) WriteBatch(ctx context.Context, batch *RecordBatch) error {
	for columnID, data := range batch.Columns {
		if err := w.WriteColumn(ctx, columnID, data); err != nil {
			return fmt.Errorf("table: %w", err)
		}
	}
	return nil
}

// Finish returns a Fragment containing all written DataFiles.
func (w *FragmentWriter) Finish() (*Fragment, error) {
	frag := NewFragment(w.fragmentID)
	frag.NumRows = w.numRows
	frag.PhysicalRows = w.numRows
	frag.DataFiles = append(frag.DataFiles, w.dataFiles...)
	return frag, nil
}
