package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalFS implements ObjectStore backed by the local filesystem.
type LocalFS struct {
	root string
}

// NewLocalFS creates a LocalFS rooted at root.
func NewLocalFS(root string) *LocalFS {
	return &LocalFS{root: root}
}

// resolve joins a relative path with the root.
func (fs *LocalFS) resolve(path string) string {
	return filepath.Join(fs.root, path)
}

// Read reads length bytes starting at offset via random access.
func (fs *LocalFS) Read(ctx context.Context, path string, offset int64, length int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	if length < 0 {
		return nil, fmt.Errorf("storage: invalid negative length %d", length)
	}
	f, err := os.Open(fs.resolve(path))
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	defer f.Close()
	buf := make([]byte, length)
	if length > 0 {
		if _, err := f.ReadAt(buf, offset); err != nil {
			return nil, fmt.Errorf("storage: %w", err)
		}
	}
	return buf, nil
}

// Write writes data to path, creating parent directories as needed.
func (fs *LocalFS) Write(ctx context.Context, path string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	full := fs.resolve(path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	return nil
}

// Delete removes the file at path.
func (fs *LocalFS) Delete(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	if err := os.Remove(fs.resolve(path)); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	return nil
}

// Exists reports whether a file exists at path.
func (fs *LocalFS) Exists(ctx context.Context, path string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("storage: %w", err)
	}
	_, err := os.Stat(fs.resolve(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("storage: %w", err)
}

// List lists files (non-recursive) in dir, returning paths relative to root.
func (fs *LocalFS) List(ctx context.Context, dir string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	entries, err := os.ReadDir(fs.resolve(dir))
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	var result []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		result = append(result, filepath.Join(dir, e.Name()))
	}
	return result, nil
}

// Size returns the size of the file at path in bytes.
func (fs *LocalFS) Size(ctx context.Context, path string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("storage: %w", err)
	}
	info, err := os.Stat(fs.resolve(path))
	if err != nil {
		return 0, fmt.Errorf("storage: %w", err)
	}
	return info.Size(), nil
}

// OpenWrite opens path for streaming writes, creating parent directories as needed.
func (fs *LocalFS) OpenWrite(ctx context.Context, path string) (io.WriteCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	full := fs.resolve(path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	f, err := os.Create(full)
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	return f, nil
}
