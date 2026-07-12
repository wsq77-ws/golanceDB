package table

import (
	"testing"

	"github.com/glancedb/glancedb/encode"
)

func TestNewManifest(t *testing.T) {
	schema := NewSchema([]*Field{{Name: "id", Type: encode.TypeInt64}})
	m := NewManifest(1, schema)

	if m.Version != 1 {
		t.Errorf("expected version 1, got %d", m.Version)
	}
	if m.Schema == nil || m.Schema.NumFields() != 1 {
		t.Errorf("expected schema with 1 field")
	}
	if m.Tags == nil {
		t.Error("expected non-nil Tags map")
	}
	if len(m.Tags) != 0 {
		t.Errorf("expected empty Tags, got %v", m.Tags)
	}
}

func TestManifestSerializeRoundtrip(t *testing.T) {
	schema := NewSchema([]*Field{
		{Name: "id", Type: encode.TypeInt64},
		{Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 128},
	})

	frag := NewFragment(0)
	frag.NumRows = 100
	frag.PhysicalRows = 100
	frag.AddDataFile(&DataFile{Path: "data/f0/col_0.lance", ColumnID: 0, NumRows: 100, FileSize: 800})
	frag.AddDataFile(&DataFile{Path: "data/f0/col_1.lance", ColumnID: 1, NumRows: 100, FileSize: 51200})

	m := NewManifest(5, schema)
	m.Timestamp = 1700000000
	m.Fragments = []*Fragment{frag}
	m.MaxFragmentID = 0
	m.IndexIDs = []string{"idx_0"}
	m.Tags["created_by"] = "test"

	data, err := m.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	got, err := DeserializeManifest(data)
	if err != nil {
		t.Fatalf("DeserializeManifest failed: %v", err)
	}

	if got.Version != m.Version {
		t.Errorf("version: expected %d, got %d", m.Version, got.Version)
	}
	if got.Timestamp != m.Timestamp {
		t.Errorf("timestamp: expected %d, got %d", m.Timestamp, got.Timestamp)
	}
	if got.Schema == nil || got.Schema.NumFields() != 2 {
		t.Errorf("schema fields: expected 2, got %v", got.Schema)
	}
	if got.Schema.FieldByName("embedding").Dimension != 128 {
		t.Errorf("embedding dimension: expected 128, got %d", got.Schema.FieldByName("embedding").Dimension)
	}
	if len(got.Fragments) != 1 {
		t.Fatalf("expected 1 fragment, got %d", len(got.Fragments))
	}
	if got.Fragments[0].ID != 0 || got.Fragments[0].NumRows != 100 {
		t.Errorf("fragment mismatch: %+v", got.Fragments[0])
	}
	if got.Fragments[0].NumDataFiles() != 2 {
		t.Errorf("expected 2 data files, got %d", got.Fragments[0].NumDataFiles())
	}
	if got.MaxFragmentID != 0 {
		t.Errorf("max fragment ID: expected 0, got %d", got.MaxFragmentID)
	}
	if len(got.IndexIDs) != 1 || got.IndexIDs[0] != "idx_0" {
		t.Errorf("index IDs mismatch: %v", got.IndexIDs)
	}
	if got.Tags["created_by"] != "test" {
		t.Errorf("tags mismatch: %v", got.Tags)
	}
}

func TestManifestSerializeEmptyFragments(t *testing.T) {
	schema := NewSchema([]*Field{{Name: "x", Type: encode.TypeInt64}})
	m := NewManifest(0, schema)

	data, err := m.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}
	got, err := DeserializeManifest(data)
	if err != nil {
		t.Fatalf("DeserializeManifest failed: %v", err)
	}
	if len(got.Fragments) != 0 {
		t.Errorf("expected 0 fragments, got %d", len(got.Fragments))
	}
}

func TestDeserializeManifestInvalid(t *testing.T) {
	if _, err := DeserializeManifest([]byte("not json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}
