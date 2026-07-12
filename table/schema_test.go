package table

import (
	"testing"

	"github.com/glancedb/glancedb/encode"
)

func TestNewSchemaAutoAssignsIDs(t *testing.T) {
	fields := []*Field{
		{Name: "id", Type: encode.TypeInt64},
		{Name: "name", Type: encode.TypeString},
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 128},
	}
	schema := NewSchema(fields)

	if schema.NumFields() != 3 {
		t.Fatalf("expected 3 fields, got %d", schema.NumFields())
	}
	for i, f := range schema.Fields {
		if f.ID != int32(i) {
			t.Errorf("field %d: expected ID %d, got %d", i, i, f.ID)
		}
	}
}

func TestNewSchemaDoesNotMutateInputs(t *testing.T) {
	fields := []*Field{
		{Name: "id", Type: encode.TypeInt64, ID: 99},
		{Name: "name", Type: encode.TypeString},
	}
	schema := NewSchema(fields)
	if schema.Fields[0].ID != 0 {
		t.Fatalf("expected schema field ID 0, got %d", schema.Fields[0].ID)
	}
	if fields[0].ID != 99 {
		t.Errorf("input field was mutated: expected ID 99, got %d", fields[0].ID)
	}
}

func TestSchemaFieldByName(t *testing.T) {
	schema := NewSchema([]*Field{
		{Name: "id", Type: encode.TypeInt64},
		{Name: "name", Type: encode.TypeString},
	})

	if f := schema.FieldByName("id"); f == nil || f.Name != "id" {
		t.Errorf("expected to find field 'id'")
	}
	if f := schema.FieldByName("missing"); f != nil {
		t.Errorf("expected nil for missing field, got %v", f)
	}
}

func TestSchemaFieldByID(t *testing.T) {
	schema := NewSchema([]*Field{
		{Name: "id", Type: encode.TypeInt64},
		{Name: "name", Type: encode.TypeString},
	})

	if f := schema.FieldByID(1); f == nil || f.Name != "name" {
		t.Errorf("expected to find field with ID 1")
	}
	if f := schema.FieldByID(99); f != nil {
		t.Errorf("expected nil for unknown ID, got %v", f)
	}
}

func TestSchemaHasField(t *testing.T) {
	schema := NewSchema([]*Field{
		{Name: "id", Type: encode.TypeInt64},
	})

	if !schema.HasField("id") {
		t.Errorf("expected HasField('id') to be true")
	}
	if schema.HasField("missing") {
		t.Errorf("expected HasField('missing') to be false")
	}
}

func TestSchemaFixedSizeListField(t *testing.T) {
	dim := int32(128)
	schema := NewSchema([]*Field{
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: dim, Nullable: false},
	})

	f := schema.FieldByName("embedding")
	if f == nil {
		t.Fatal("expected to find embedding field")
	}
	if f.Type != encode.TypeFixedSizeList {
		t.Errorf("expected TypeFixedSizeList, got %d", f.Type)
	}
	if f.Dimension != dim {
		t.Errorf("expected dimension %d, got %d", dim, f.Dimension)
	}
}

func TestSchemaNumFields(t *testing.T) {
	schema := NewSchema([]*Field{})
	if schema.NumFields() != 0 {
		t.Errorf("expected 0 fields, got %d", schema.NumFields())
	}

	schema2 := NewSchema([]*Field{
		{Name: "a", Type: encode.TypeInt64},
		{Name: "b", Type: encode.TypeInt32},
	})
	if schema2.NumFields() != 2 {
		t.Errorf("expected 2 fields, got %d", schema2.NumFields())
	}
}
