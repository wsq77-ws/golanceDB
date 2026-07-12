package table

import (
	"encoding/json"
	"fmt"
)

// Manifest records the metadata of a table version.
type Manifest struct {
	Version       int64
	Timestamp     int64
	Schema        *Schema
	Fragments     []*Fragment
	IndexIDs      []string
	MaxFragmentID int32
	Tags          map[string]string
}

// NewManifest creates a Manifest for the given version and schema.
func NewManifest(version int64, schema *Schema) *Manifest {
	return &Manifest{
		Version:   version,
		Schema:    schema,
		Fragments: nil,
		IndexIDs:  nil,
		Tags:      make(map[string]string),
	}
}

// Serialize marshals the manifest to JSON bytes.
func (m *Manifest) Serialize() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	return data, nil
}

// DeserializeManifest unmarshals a manifest from JSON bytes.
func DeserializeManifest(data []byte) (*Manifest, error) {
	m := &Manifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	return m, nil
}
