package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Path != "./golancedb_data" {
		t.Errorf("expected default path, got %q", cfg.Path)
	}
	if cfg.Storage.Backend != BackendLocal {
		t.Errorf("expected local backend, got %q", cfg.Storage.Backend)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "glancedb.json")

	cfg := DefaultConfig()
	cfg.Path = filepath.Join(dir, "mydb")
	cfg.Storage.Backend = BackendLocal

	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Path != cfg.Path {
		t.Errorf("expected path %q, got %q", cfg.Path, loaded.Path)
	}
	if loaded.Storage.Backend != BackendLocal {
		t.Errorf("expected backend local, got %q", loaded.Storage.Backend)
	}
}

func TestLoadNonExistentReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	nonexistent := filepath.Join(dir, "no-such-file.json")

	cfg, err := Load(nonexistent)
	if err != nil {
		t.Fatalf("Load should not error for missing file: %v", err)
	}
	if cfg.Path != "./golancedb_data" {
		t.Errorf("expected default path for missing file, got %q", cfg.Path)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{"valid local", &Config{Path: "/data", Storage: StorageConfig{Backend: BackendLocal}}, false},
		{"valid default", &Config{Path: "/data"}, false},
		{"missing path", &Config{}, true},
		{"FUSE missing mount", &Config{Path: "/data", Storage: StorageConfig{Backend: BackendFUSE}}, true},
		{"FUSE ok", &Config{Path: "/data", Storage: StorageConfig{Backend: BackendFUSE, FUSE: FUSEConfig{MountPoint: "/mnt/fuse"}}}, false},
		{"S3 missing bucket", &Config{Path: "/data", Storage: StorageConfig{Backend: BackendS3, S3: S3Config{Region: "us-east-1"}}}, true},
		{"S3 missing region", &Config{Path: "/data", Storage: StorageConfig{Backend: BackendS3, S3: S3Config{Bucket: "mybucket"}}}, true},
		{"unknown backend", &Config{Path: "/data", Storage: StorageConfig{Backend: "gcs"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewStoreLocal(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Path = dir

	store, err := cfg.NewStore()
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	// Verify the store works by writing and reading.
	ctx := context.Background()
	testPath := filepath.Join(dir, "_test_file")
	if err := store.Write(ctx, testPath, []byte("hello")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	data, err := store.Read(ctx, testPath, 0, 5)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestNewStoreUnknownBackend(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.Backend = "gcs"

	_, err := cfg.NewStore()
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestSaveCreatesDir(t *testing.T) {
	dir := t.TempDir()
	// Save into a subdirectory that doesn't exist yet.
	cfgPath := filepath.Join(dir, "sub", "dir", "cfg.json")

	cfg := DefaultConfig()
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("saved file does not exist after Save")
	}
}
