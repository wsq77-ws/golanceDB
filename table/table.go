package table

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/storage"
)

// Table represents a database table with MVCC version control and concurrent access.
type Table struct {
	name          string
	tablePath     string
	schema        *Schema
	store         storage.ObjectStore
	manifestStore *ManifestStore
	versionMgr    *VersionManager
	compression   encode.CompressionType

	mu             sync.RWMutex // protects schema, nextFragmentID, nextVersion
	nextFragmentID int32
	nextVersion    int64
}

// NewTable creates a new Table instance. It does NOT create files on disk.
// Call Create() to initialize a new table on disk, or Open() to load an existing one.
func NewTable(name string, tablePath string, schema *Schema, store storage.ObjectStore, compression encode.CompressionType) *Table {
	return &Table{
		name:           name,
		tablePath:      tablePath,
		schema:         schema,
		store:          store,
		manifestStore:  NewManifestStore(store, filepath.Join(tablePath, "_versions")),
		versionMgr:     NewVersionManager(10),
		compression:    compression,
		nextVersion:    1,
		nextFragmentID: 0,
	}
}

// Create initializes a new table on disk with an initial manifest (version 1).
func Create(ctx context.Context, name string, tablePath string, schema *Schema, store storage.ObjectStore, compression encode.CompressionType) (*Table, error) {
	t := NewTable(name, tablePath, schema, store, compression)

	manifest := NewManifest(t.nextVersion, schema)
	manifest.MaxFragmentID = 0
	manifest.Timestamp = time.Now().UnixNano()

	if err := t.manifestStore.Write(ctx, manifest); err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	t.nextVersion++
	t.versionMgr.Publish(manifest)

	return t, nil
}

// Open loads an existing table from disk by reading the latest manifest.
func Open(ctx context.Context, name string, tablePath string, store storage.ObjectStore, compression encode.CompressionType) (*Table, error) {
	t := &Table{
		name:           name,
		tablePath:      tablePath,
		store:          store,
		manifestStore:  NewManifestStore(store, filepath.Join(tablePath, "_versions")),
		versionMgr:     NewVersionManager(10),
		compression:    compression,
		nextVersion:    1,
		nextFragmentID: 0,
	}

	latest, err := t.manifestStore.LatestVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}

	manifest, err := t.manifestStore.Read(ctx, latest)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}

	t.schema = manifest.Schema
	t.nextVersion = latest + 1
	t.nextFragmentID = manifest.MaxFragmentID + 1
	t.versionMgr.Publish(manifest)

	return t, nil
}

// Schema returns the current schema. Safe for concurrent access.
func (t *Table) Schema() *Schema {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.schema
}

// Name returns the table name.
func (t *Table) Name() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.name
}

// LatestManifest returns the latest manifest from the in-memory cache.
// If not in cache, loads from disk.
func (t *Table) LatestManifest(ctx context.Context) (*Manifest, error) {
	t.mu.RLock()
	m, ok := t.versionMgr.Latest()
	t.mu.RUnlock()
	if ok {
		return m, nil
	}

	latest, err := t.manifestStore.LatestVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	m, err = t.manifestStore.Read(ctx, latest)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	return m, nil
}

// CheckoutVersion returns the manifest for a specific version.
// Checks in-memory cache first, then falls back to disk.
func (t *Table) CheckoutVersion(ctx context.Context, version int64) (*Manifest, error) {
	t.mu.RLock()
	m, ok := t.versionMgr.Get(version)
	t.mu.RUnlock()
	if ok {
		return m, nil
	}

	m, err := t.manifestStore.Read(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	return m, nil
}

// NumFragments returns the number of fragments in the latest manifest.
func (t *Table) NumFragments(ctx context.Context) (int, error) {
	m, err := t.LatestManifest(ctx)
	if err != nil {
		return 0, err
	}
	return len(m.Fragments), nil
}

// NumRows returns the total number of rows across all fragments in the latest manifest.
func (t *Table) NumRows(ctx context.Context) (int64, error) {
	m, err := t.LatestManifest(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, f := range m.Fragments {
		total += f.NumRows
	}
	return total, nil
}

// ListVersions returns all available versions on disk.
func (t *Table) ListVersions(ctx context.Context) ([]int64, error) {
	return t.manifestStore.ListVersions(ctx)
}

// Insert writes a RecordBatch as a new fragment and commits a new manifest version.
func (t *Table) Insert(ctx context.Context, batch *RecordBatch) (*Fragment, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	fragmentID := t.nextFragmentID
	writer := NewFragmentWriter(t.store, t.schema, fragmentID, t.tablePath, t.compression)
	if err := writer.WriteBatch(ctx, batch); err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	frag, err := writer.Finish()
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	t.nextFragmentID++

	latest, ok := t.versionMgr.Latest()
	if !ok {
		return nil, fmt.Errorf("table: no existing manifest to append to")
	}

	newFragments := make([]*Fragment, 0, len(latest.Fragments)+1)
	newFragments = append(newFragments, latest.Fragments...)
	newFragments = append(newFragments, frag)

	version := t.nextVersion
	t.nextVersion++

	newManifest := NewManifest(version, t.schema)
	newManifest.Fragments = newFragments
	newManifest.MaxFragmentID = fragmentID
	newManifest.Timestamp = time.Now().UnixNano()

	if err := t.manifestStore.Write(ctx, newManifest); err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	t.versionMgr.Publish(newManifest)

	return frag, nil
}

// AddColumn adds a new column to the schema (zero-copy evolution).
// Existing fragments do not need rewriting — they simply lack the new column's DataFile.
func (t *Table) AddColumn(ctx context.Context, field *Field) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.schema.HasField(field.Name) {
		return fmt.Errorf("table: field %q already exists", field.Name)
	}

	var maxID int32
	for _, f := range t.schema.Fields {
		if f.ID > maxID {
			maxID = f.ID
		}
	}

	newFields := make([]*Field, 0, len(t.schema.Fields)+1)
	for _, f := range t.schema.Fields {
		cp := *f
		newFields = append(newFields, &cp)
	}
	newField := *field
	newField.ID = maxID + 1
	newFields = append(newFields, &newField)
	newSchema := &Schema{Fields: newFields}

	latest, ok := t.versionMgr.Latest()
	if !ok {
		return fmt.Errorf("table: no existing manifest")
	}

	newFragments := make([]*Fragment, len(latest.Fragments))
	copy(newFragments, latest.Fragments)

	version := t.nextVersion
	t.nextVersion++

	newManifest := NewManifest(version, newSchema)
	newManifest.Fragments = newFragments
	newManifest.MaxFragmentID = latest.MaxFragmentID
	newManifest.Timestamp = time.Now().UnixNano()

	if err := t.manifestStore.Write(ctx, newManifest); err != nil {
		return fmt.Errorf("table: %w", err)
	}
	t.schema = newSchema
	t.versionMgr.Publish(newManifest)

	return nil
}

// DropColumn removes a column from the schema (zero-copy evolution).
// Existing data files are not deleted.
func (t *Table) DropColumn(ctx context.Context, columnName string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.schema.FieldByName(columnName) == nil {
		return fmt.Errorf("table: column %q not found", columnName)
	}

	newFields := make([]*Field, 0, len(t.schema.Fields)-1)
	for _, f := range t.schema.Fields {
		if f.Name != columnName {
			cp := *f
			newFields = append(newFields, &cp)
		}
	}
	newSchema := &Schema{Fields: newFields}

	latest, ok := t.versionMgr.Latest()
	if !ok {
		return fmt.Errorf("table: no existing manifest")
	}

	newFragments := make([]*Fragment, len(latest.Fragments))
	copy(newFragments, latest.Fragments)

	version := t.nextVersion
	t.nextVersion++

	newManifest := NewManifest(version, newSchema)
	newManifest.Fragments = newFragments
	newManifest.MaxFragmentID = latest.MaxFragmentID
	newManifest.Timestamp = time.Now().UnixNano()

	if err := t.manifestStore.Write(ctx, newManifest); err != nil {
		return fmt.Errorf("table: %w", err)
	}
	t.schema = newSchema
	t.versionMgr.Publish(newManifest)

	return nil
}

// Close flushes any pending state (currently a no-op since we don't have async writer here).
func (t *Table) Close() error {
	return nil
}
