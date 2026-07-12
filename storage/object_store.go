package storage

import (
	"context"
	"io"
)

// ObjectStore abstracts storage backends (local FS, S3, etc).
type ObjectStore interface {
	Read(ctx context.Context, path string, offset int64, length int64) ([]byte, error)
	Write(ctx context.Context, path string, data []byte) error
	Delete(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
	List(ctx context.Context, dir string) ([]string, error)
	Size(ctx context.Context, path string) (int64, error)
	OpenWrite(ctx context.Context, path string) (io.WriteCloser, error)
}
