// Package config provides configuration and storage backend factory for GlanceDB.
//
// Usage:
//
//	// Load from file
//	cfg, err := config.Load("glancedb.json")
//
//	// Use defaults
//	cfg := config.DefaultConfig()
//	cfg.Path = "/data/my-db"
//
//	// Create store from config
//	store, err := cfg.NewStore()
//
//	// Connect with config
//	db, err := api.ConnectWithConfig(ctx, cfg)
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glancedb/glancedb/storage"
)

// BackendType defines the storage backend to use.
type BackendType string

const (
	// BackendLocal uses the local filesystem (default).
	BackendLocal BackendType = "local"
	// BackendFUSE uses a FUSE filesystem mount point.
	BackendFUSE BackendType = "fuse"
	// BackendS3 uses an S3-compatible object store.
	BackendS3 BackendType = "s3"
)

// Config is the top-level configuration for GlanceDB.
type Config struct {
	// Path is the database directory (required for local backend, used as
	// root path for other backends).
	Path string `json:"path"`

	// Storage configures the storage backend.
	Storage StorageConfig `json:"storage"`
}

// StorageConfig configures a storage backend.
type StorageConfig struct {
	// Backend selects the storage driver: "local" (default), "fuse", or "s3".
	Backend BackendType `json:"backend"`

	// Local configures local filesystem storage.
	Local LocalConfig `json:"local"`

	// S3 configures S3-compatible object storage.
	S3 S3Config `json:"s3"`

	// FUSE configures FUSE filesystem storage.
	FUSE FUSEConfig `json:"fuse"`

	// BufferPoolSize is the LRU cache size in bytes (0 = disabled).
	// Applied only when the backend supports local buffering.
	BufferPoolSize int64 `json:"buffer_pool_size"`
}

// LocalConfig configures local filesystem storage.
type LocalConfig struct {
	// Root is the filesystem root (empty = use database path directly).
	Root string `json:"root"`
}

// S3Config configures S3-compatible object storage.
// This is a placeholder for future implementation.
type S3Config struct {
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
	Region          string `json:"region"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

// FUSEConfig configures FUSE filesystem storage.
// This is a placeholder for future implementation.
type FUSEConfig struct {
	MountPoint string `json:"mount_point"`
}

// DefaultConfig returns a Config with sensible defaults for local development.
func DefaultConfig() *Config {
	return &Config{
		Path: "./golancedb_data",
		Storage: StorageConfig{
			Backend: BackendLocal,
			Local: LocalConfig{
				Root: "",
			},
		},
	}
}

// Load reads configuration from a JSON file.
// Returns the default config if the file does not exist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			return cfg, nil
		}
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %q: %w", path, err)
	}
	return cfg, nil
}

// Save writes the configuration to a JSON file.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: create dir %q: %w", dir, err)
		}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: serialize: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("config: write %q: %w", path, err)
	}
	return nil
}

// NewStore creates a storage.Store from the configuration.
// It returns an error if the backend type is unknown or missing required fields.
//
// For the local backend, the returned Store roots all paths at Local.Root
// (or the current working directory if empty). The Config.Path serves as the
// logical database path and should be passed as an absolute or relative path
// when calling Store methods.
func (c *Config) NewStore() (storage.Store, error) {
	switch c.Storage.Backend {
	case BackendLocal, "":
		return storage.NewLocalFS(c.Storage.Local.Root), nil

	case BackendFUSE:
		if c.Storage.FUSE.MountPoint == "" {
			return nil, fmt.Errorf("config: FUSE mount_point is required")
		}
		return storage.NewLocalFS(c.Storage.FUSE.MountPoint), nil

	case BackendS3:
		if c.Storage.S3.Bucket == "" {
			return nil, fmt.Errorf("config: S3 bucket is required")
		}
		if c.Storage.S3.Region == "" {
			return nil, fmt.Errorf("config: S3 region is required")
		}
		return nil, fmt.Errorf("config: S3 backend not yet implemented")

	default:
		return nil, fmt.Errorf("config: unknown storage backend %q", c.Storage.Backend)
	}
}

// Validate checks the configuration for common errors.
func (c *Config) Validate() error {
	if c.Path == "" {
		return fmt.Errorf("config: path is required")
	}
	switch c.Storage.Backend {
	case BackendLocal, "":
		// OK
	case BackendFUSE:
		if c.Storage.FUSE.MountPoint == "" {
			return fmt.Errorf("config: FUSE storage requires mount_point")
		}
	case BackendS3:
		if c.Storage.S3.Bucket == "" {
			return fmt.Errorf("config: S3 storage requires bucket")
		}
		if c.Storage.S3.Region == "" {
			return fmt.Errorf("config: S3 storage requires region")
		}
	default:
		return fmt.Errorf("config: unknown backend %q", c.Storage.Backend)
	}
	return nil
}
