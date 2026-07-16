package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/glancedb/glancedb/storage"
)

// Database is the top-level handle for a GlanceDB instance.
// It manages table lifecycle and storage connectivity. Use Connect to create one.
type Database struct {
	path   string
	store  storage.Store
	tables map[string]*tableRef
	mu     sync.RWMutex
	logger *Logger
	closed bool
}

type tableRef struct {
	*Table
}

// Connect opens or creates a GlanceDB database at the given directory path.
// The directory is created if it does not exist. Uses the local filesystem.
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

	tableDir := filepath.Join(db.path, name)
	if err := os.RemoveAll(tableDir); err != nil {
		return e(ErrStorage, "database.DropTable",
			fmt.Sprintf("cannot remove table directory %q", tableDir), err)
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
