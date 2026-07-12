package table

import (
	"context"
	"fmt"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/storage"
)

// FragmentReader reads column data from .lance files.
type FragmentReader struct {
	store   storage.ObjectStore
	schema  *Schema
	encoder encode.ColumnEncoder
}

// NewFragmentReader creates a FragmentReader with the default MiniBlock encoder.
func NewFragmentReader(store storage.ObjectStore, schema *Schema) *FragmentReader {
	return &FragmentReader{
		store:   store,
		schema:  schema,
		encoder: encode.NewMiniBlockEncoder(encode.CompressionNone),
	}
}

// findDataFile returns the DataFile for the given column ID, or nil if not found.
func findDataFile(fragment *Fragment, columnID int32) *DataFile {
	for _, df := range fragment.DataFiles {
		if df.ColumnID == columnID {
			return df
		}
	}
	return nil
}

// ReadColumn reads and decodes a single column from the fragment.
func (r *FragmentReader) ReadColumn(ctx context.Context, fragment *Fragment, columnID int32) (interface{}, error) {
	field := r.schema.FieldByID(columnID)
	if field == nil {
		return nil, fmt.Errorf("table: field with ID %d not found in schema", columnID)
	}

	df := findDataFile(fragment, columnID)
	if df == nil {
		return nil, fmt.Errorf("table: data file for column %d not found in fragment %d", columnID, fragment.ID)
	}

	data, err := r.store.Read(ctx, df.Path, 0, df.FileSize)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}

	decoded, err := r.encoder.Decode(data, field.Type, df.NumRows)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	return decoded, nil
}

// ReadBatch reads multiple columns and returns them as a RecordBatch.
func (r *FragmentReader) ReadBatch(ctx context.Context, fragment *Fragment, columnIDs []int32) (*RecordBatch, error) {
	numRows := fragment.PhysicalRows
	if numRows == 0 {
		numRows = fragment.NumRows
	}
	batch := NewRecordBatch(r.schema, numRows)
	for _, id := range columnIDs {
		data, err := r.ReadColumn(ctx, fragment, id)
		if err != nil {
			return nil, fmt.Errorf("table: %w", err)
		}
		batch.SetColumn(id, data)
	}
	return batch, nil
}
