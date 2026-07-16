package api

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/query"
	"github.com/glancedb/glancedb/storage"
	"github.com/glancedb/glancedb/table"
)

// TableSchema is an alias for table.Schema to avoid importing table package.
type TableSchema = table.Schema

// Table wraps the underlying table.Table with a developer-friendly API,
// providing Insert (sync + async) and Search (vector + hybrid) operations.
type Table struct {
	name   string
	table  *table.Table
	store  storage.Store
	logger *Logger
	dbPath string

	mu       sync.RWMutex
	search   *query.HybridSearch
	writer   *table.AsyncWriter
	closed   bool
	writerOn bool
}

// newTable creates and initializes a new table on disk.
func newTable(ctx context.Context, db *Database, name string, schema *TableSchema) (*Table, error) {
	tableDir := filepath.Join(db.path, name)
	tbl, err := table.Create(ctx, name, tableDir, schema, db.store, encode.CompressionZstd)
	if err != nil {
		return nil, wrapStorageErr(err, "table.Create", name)
	}
	return wrapTable(tbl, db, name, tableDir), nil
}

// openTable opens an existing table from disk.
func openTable(ctx context.Context, db *Database, name string) (*Table, error) {
	tableDir := filepath.Join(db.path, name)
	tbl, err := table.Open(ctx, name, tableDir, db.store, encode.CompressionZstd)
	if err != nil {
		return nil, wrapStorageErr(err, "table.Open", name)
	}
	return wrapTable(tbl, db, name, tableDir), nil
}

func wrapTable(tbl *table.Table, db *Database, name, tableDir string) *Table {
	t := &Table{
		name:   name,
		table:  tbl,
		store:  db.store,
		logger: db.logger.WithTable(name),
		dbPath: tableDir,
	}
	t.refreshSearch()
	return t
}

// refreshSearch rebuilds the HybridSearch components with the current schema.
// Must be called after schema changes (AddColumn/DropColumn) so that search
// uses the up-to-date field definitions.
func (t *Table) refreshSearch() {
	schema := t.table.Schema()
	reader := table.NewFragmentReader(t.store, schema, encode.CompressionZstd)
	t.search = query.NewHybridSearch(
		query.NewBruteForceSearch(reader, schema),
		query.NewScanFilter(reader, schema),
	)
}

// Name returns the table name.
func (t *Table) Name() string { return t.name }

// Schema returns the current table schema.
func (t *Table) Schema() *TableSchema { return t.table.Schema() }

// NumRows returns the total number of rows in the table.
// It reads the latest manifest from disk to include data written by async writers.
func (t *Table) NumRows(ctx context.Context) (int64, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return 0, ErrTableClosed
	}
	t.mu.RUnlock()

	manifest, err := t.table.LatestManifest(ctx)
	if err != nil {
		return 0, wrapStorageErr(err, "table.NumRows", t.name)
	}
	var total int64
	for _, f := range manifest.Fragments {
		total += f.NumRows
	}
	return total, nil
}

// Insert synchronously writes a RecordBatch and commits a new manifest version.
// It blocks until the write and metadata commit complete.
func (t *Table) Insert(ctx context.Context, batch *table.RecordBatch) error {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return ErrTableClosed
	}
	t.mu.RUnlock()

	return t.logger.LogOperation(ctx, "table.Insert", func(ctx context.Context) error {
		frag, err := t.table.BatchInsert(ctx, []*table.RecordBatch{batch})
		if err != nil {
			return wrapStorageErr(err, "table.Insert", t.name)
		}
		t.logger.DebugContext(ctx, "insert completed",
			"fragment_id", frag.ID,
			"num_rows", batch.NumRows,
			"columns", len(batch.Columns),
		)
		return nil
	})
}

// BatchInsert writes multiple RecordBatches as a single fragment.
// All batches are merged into one fragment and committed with a single manifest
// version, drastically reducing write amplification for bulk operations.
func (t *Table) BatchInsert(ctx context.Context, batches []*table.RecordBatch) error {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return ErrTableClosed
	}
	t.mu.RUnlock()

	var totalRows int64
	for _, b := range batches {
		totalRows += b.NumRows
	}

	return t.logger.LogOperation(ctx, "table.BatchInsert", func(ctx context.Context) error {
		frag, err := t.table.BatchInsert(ctx, batches)
		if err != nil {
			return wrapStorageErr(err, "table.BatchInsert", t.name)
		}
		t.logger.DebugContext(ctx, "batch insert completed",
			"fragment_id", frag.ID,
			"num_rows", totalRows,
			"num_batches", len(batches),
		)
		return nil
	})
}

// StartAsyncWriter starts the async batch writer with the given buffer size and flush interval.
// After starting, InsertAsync can be used for non-blocking writes.
func (t *Table) StartAsyncWriter(ctx context.Context, maxBatchSize int, flushInterval time.Duration) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrTableClosed
	}
	if t.writerOn {
		return nil // already started
	}

	t.writer = table.NewAsyncWriter(
		t.store, t.table.ManifestStore(),
		t.table.Schema(), t.dbPath,
		encode.CompressionZstd, maxBatchSize, flushInterval,
	)
	if err := t.writer.Start(ctx); err != nil {
		return wrapStorageErr(err, "table.StartAsyncWriter", t.name)
	}
	t.writerOn = true
	t.logger.InfoContext(ctx, "async writer started",
		"max_batch_size", maxBatchSize,
		"flush_interval", flushInterval.String(),
	)
	return nil
}

// InsertAsync enqueues a batch for asynchronous write.
// Requires StartAsyncWriter to have been called first.
func (t *Table) InsertAsync(ctx context.Context, batch *table.RecordBatch) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return ErrTableClosed
	}
	if !t.writerOn {
		return ErrWriterStopped
	}
	return t.writer.Write(ctx, batch)
}

// Flush triggers a synchronous flush of any pending async writes.
func (t *Table) Flush(ctx context.Context) error {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return ErrTableClosed
	}
	if !t.writerOn {
		t.mu.RUnlock()
		return nil
	}
	t.mu.RUnlock()

	return t.writer.Flush()
}

// Search executes a vector or hybrid query and returns results.
func (t *Table) Search(ctx context.Context, q *query.Query) ([]query.SearchResult, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, ErrTableClosed
	}
	search := t.search
	t.mu.RUnlock()

	var results []query.SearchResult
	err := t.logger.LogOperation(ctx, "table.Search", func(ctx context.Context) error {
		manifest, err := t.table.LatestManifest(ctx)
		if err != nil {
			return wrapStorageErr(err, "table.Search.LatestManifest", t.name)
		}
		results, err = search.Search(ctx, manifest, q)
		if err != nil {
			return wrapStorageErr(err, "table.Search", t.name)
		}
		return nil
	})
	return results, err
}

// AddColumn adds a new column (zero-copy evolution).
func (t *Table) AddColumn(ctx context.Context, field *table.Field) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return ErrTableClosed
	}
	if err := t.table.AddColumn(ctx, field); err != nil {
		return wrapStorageErr(err, "table.AddColumn", t.name)
	}
	t.refreshSearch()
	return nil
}

// DropColumn removes a column from the schema.
func (t *Table) DropColumn(ctx context.Context, columnName string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return ErrTableClosed
	}
	if err := t.table.DropColumn(ctx, columnName); err != nil {
		return wrapStorageErr(err, "table.DropColumn", t.name)
	}
	t.refreshSearch()
	return nil
}

// ManifestStore returns the underlying ManifestStore for version management.
func (t *Table) ManifestStore() *table.ManifestStore {
	return t.table.ManifestStore()
}

// Close flushes pending writes and releases table resources.
func (t *Table) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	var firstErr error
	if t.writerOn && t.writer != nil {
		if err := t.writer.Close(); err != nil {
			firstErr = fmt.Errorf("table.Close: %w", err)
			t.logger.WarnContext(context.Background(), "error closing async writer", "error", err)
		}
	}
	t.logger.InfoContext(context.Background(), "table closed")
	return firstErr
}

// wrapStorageErr converts a lower-level error to an api.Error with ErrStorage code.
// It logs the original error for debugging and returns a user-friendly error.
func wrapStorageErr(err error, op, tableName string) error {
	if err == nil {
		return nil
	}
	// If already an api.Error, preserve it.
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr
	}
	// Otherwise wrap as storage error.
	logger := L().WithTable(tableName)
	logger.WarnContext(context.Background(), "storage operation failed",
		"op", op,
		"error", err.Error(),
	)
	return e(ErrStorage, op,
		fmt.Sprintf("storage operation failed for table %q", tableName), err)
}
