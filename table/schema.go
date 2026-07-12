package table

import "github.com/glancedb/glancedb/encode"

// Field describes a single column in a schema.
type Field struct {
	ID        int32
	Name      string
	Type      encode.DataType
	Nullable  bool
	Dimension int32
	Children  []*Field
	Metadata  map[string]string
}

// Schema describes the structure of a table.
type Schema struct {
	Fields []*Field
}

// NewSchema creates a schema from fields, auto-assigning sequential IDs starting at 0.
func NewSchema(fields []*Field) *Schema {
	s := &Schema{Fields: make([]*Field, len(fields))}
	for i, f := range fields {
		field := *f
		field.ID = int32(i)
		s.Fields[i] = &field
	}
	return s
}

// FieldByName returns the field with the given name, or nil if not found.
func (s *Schema) FieldByName(name string) *Field {
	for _, f := range s.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// FieldByID returns the field with the given ID, or nil if not found.
func (s *Schema) FieldByID(id int32) *Field {
	for _, f := range s.Fields {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// HasField checks if a field exists by name.
func (s *Schema) HasField(name string) bool {
	return s.FieldByName(name) != nil
}

// NumFields returns the number of fields.
func (s *Schema) NumFields() int {
	return len(s.Fields)
}
