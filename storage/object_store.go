package storage

import (
	"context"
	"io"
)

// Store abstracts storage backends (local FS, FUSE filesystem, object storage, etc).
// All table and manifest operations go through this single interface.
type Store interface {
	// Read reads length bytes starting at offset via random access.
	Read(ctx context.Context, path string, offset int64, length int64) ([]byte, error)
	// Write writes data to path, creating parent directories as needed.
	Write(ctx context.Context, path string, data []byte) error
	// Delete removes the file at path.
	Delete(ctx context.Context, path string) error
	// Exists reports whether a file or directory exists at path.
	Exists(ctx context.Context, path string) (bool, error)
	// List lists files (non-recursive) in dir, returning paths relative to root.
	List(ctx context.Context, dir string) ([]string, error)
	// Size returns the size of the file at path in bytes.
	Size(ctx context.Context, path string) (int64, error)
	// OpenWrite opens path for streaming writes, creating parent directories as needed.
	OpenWrite(ctx context.Context, path string) (io.WriteCloser, error)
}

// ObjectStore is a type alias for backward compatibility.
// New code should use storage.Store directly.
type ObjectStore = Store
