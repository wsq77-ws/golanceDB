package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/glancedb/glancedb/config"
	"github.com/glancedb/glancedb/storage"
)

// Database is the top-level handle for a GlanceDB instance.
// It manages table lifecycle and storage connectivity. Use Connect or
// ConnectWithConfig to create one.
type Database struct {
	path   string // logical database path
	store  storage.Store
	tables map[string]*tableRef
	mu     sync.RWMutex
	logger *Logger
	closed bool
}

type tableRef struct {
	*Table
}

// Connect opens or creates a GlanceDB database at the given directory path,
// using the local filesystem. This is a convenience wrapper around
// ConnectWithConfig with default local settings.
func Connect(path string) (*Database, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, e(ErrInvalidArg, "database.Connect",
			fmt.Sprintf("invalid path %q", path), err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if mkErr := os.MkdirAll(abs, 0755); mkErr != nil {
				return nil, e(ErrStorage, "database.Connect",
					fmt.Sprintf("cannot create database directory %q", abs), mkErr)
			}
		} else {
			return nil, e(ErrStorage, "database.Connect",
				fmt.Sprintf("cannot access database directory %q", abs), err)
		}
	} else if !info.IsDir() {
		return nil, e(ErrInvalidArg, "database.Connect",
			fmt.Sprintf("path %q is not a directory", abs), nil)
	}

	db := &Database{
		path:   abs,
		store:  storage.NewLocalFS(""),
		tables: make(map[string]*tableRef),
		logger: &Logger{L().With("db_path", abs)},
	}

	// Scan existing tables (directories containing _versions).
	entries, err := os.ReadDir(abs)
	if err != nil {
		db.logger.WarnContext(context.Background(), "cannot scan database directory", "error", err)
	} else {
		for _, entry := range entries {
			if entry.IsDir() {
				verDir := filepath.Join(abs, entry.Name(), "_versions")
				if _, statErr := os.Stat(verDir); statErr == nil {
					db.tables[entry.Name()] = &tableRef{}
					db.logger.DebugContext(context.Background(), "found existing table", "table", entry.Name())
				}
			}
		}
	}

	db.logger.InfoContext(context.Background(), "database connected", "path", abs)
	return db, nil
}

// ConnectWithConfig opens or creates a GlanceDB database using the given
// configuration. The storage backend is determined by cfg.Storage.Backend.
func ConnectWithConfig(ctx context.Context, cfg *config.Config) (*Database, error) {
	if err := cfg.Validate(); err != nil {
		return nil, e(ErrInvalidArg, "database.ConnectWithConfig",
			"invalid configuration", err)
	}

	store, err := cfg.NewStore()
	if err != nil {
		return nil, e(ErrInvalidArg, "database.ConnectWithConfig",
			"cannot create storage backend", err)
	}

	db := &Database{
		path:   cfg.Path,
		store:  store,
		tables: make(map[string]*tableRef),
		logger: &Logger{L().With("db_path", cfg.Path, "backend", string(cfg.Storage.Backend))},
	}

	// Scan for existing tables by listing directories that contain _versions.
	// Uses the Store interface so it works with any backend.
	exists, err := store.Exists(ctx, cfg.Path)
	if err != nil {
		db.logger.WarnContext(ctx, "cannot check database path via store", "error", err)
	} else if exists {
		entries, listErr := store.List(ctx, cfg.Path)
		if listErr != nil {
			db.logger.WarnContext(ctx, "cannot list database directory via store", "error", listErr)
		} else {
			for _, entry := range entries {
				verPath := filepath.Join(cfg.Path, entry, "_versions")
				verExists, verErr := store.Exists(ctx, verPath)
				if verErr == nil && verExists {
					db.tables[entry] = &tableRef{}
					db.logger.DebugContext(ctx, "found existing table", "table", entry)
				}
			}
		}
	}

	db.logger.InfoContext(ctx, "database connected",
		"path", cfg.Path,
		"backend", string(cfg.Storage.Backend),
		"tables_found", len(db.tables))
	return db, nil
}

// Close closes the database and releases all resources.
func (db *Database) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}
	db.closed = true

	// Close all open tables.
	for name, ref := range db.tables {
		if ref.Table != nil {
			if err := ref.Table.Close(); err != nil {
				db.logger.WarnContext(context.Background(), "error closing table", "table", name, "error", err)
			}
		}
	}
	db.tables = nil
	db.logger.InfoContext(context.Background(), "database closed")
	return nil
}

// CreateTable creates a new table with the given name and schema.
// Returns ErrConflict if a table with that name already exists.
func (db *Database) CreateTable(ctx context.Context, name string, schema *TableSchema) (*Table, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil, ErrDBClosed
	}
	if _, exists := db.tables[name]; exists {
		return nil, e(ErrConflict, "database.CreateTable",
			fmt.Sprintf("table %q already exists", name), nil)
	}

	tbl, err := newTable(ctx, db, name, schema)
	if err != nil {
		return nil, err
	}
	db.tables[name] = &tableRef{Table: tbl}
	db.logger.InfoContext(ctx, "table created", "table", name, "fields", len(schema.Fields))
	return tbl, nil
}

// OpenTable opens an existing table by name.
// Returns ErrNotFound if the table does not exist.
func (db *Database) OpenTable(ctx context.Context, name string) (*Table, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil, ErrDBClosed
	}

	ref, exists := db.tables[name]
	if !exists {
		return nil, e(ErrNotFound, "database.OpenTable",
			fmt.Sprintf("table %q not found", name), nil)
	}
	if ref.Table != nil {
		return ref.Table, nil
	}

	tbl, err := openTable(ctx, db, name)
	if err != nil {
		return nil, err
	}
	ref.Table = tbl
	return tbl, nil
}

// DropTable drops a table by removing its data directory.
// Returns ErrNotFound if the table does not exist.
func (db *Database) DropTable(ctx context.Context, name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDBClosed
	}

	ref, exists := db.tables[name]
	if !exists {
		return e(ErrNotFound, "database.DropTable",
			fmt.Sprintf("table %q not found", name), nil)
	}
	if ref.Table != nil {
		if err := ref.Table.Close(); err != nil {
			return err
		}
	}

	// Remove the table directory via the Store interface.
	tableDir := filepath.Join(db.path, name)
	if err := db.store.Delete(ctx, tableDir); err != nil {
		// For local FS, attempt recursive removal.
		if err2 := os.RemoveAll(tableDir); err2 != nil {
			return e(ErrStorage, "database.DropTable",
				fmt.Sprintf("cannot remove table directory %q", tableDir), err)
		}
	}
	delete(db.tables, name)
	db.logger.InfoContext(ctx, "table dropped", "table", name)
	return nil
}

// ListTables returns the names of all tables in the database.
func (db *Database) ListTables(ctx context.Context) ([]string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, ErrDBClosed
	}

	names := make([]string, 0, len(db.tables))
	for name := range db.tables {
		names = append(names, name)
	}
	return names, nil
}

// Path returns the database directory path.
func (db *Database) Path() string { return db.path }

// Store returns the underlying storage backend.
func (db *Database) Store() storage.Store { return db.store }
