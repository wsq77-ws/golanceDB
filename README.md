# GolanceDB

**A Go-based embedded vector database engine built on the Lance columnar format.**

GolanceDB is a pure Go embedded vector database engine compatible with the Lance columnar storage format. Designed for AI/ML workloads, it provides columnar storage, vector similarity search, scalar filtering, and MVCC versioning — zero CGO dependencies, zero external runtime dependencies.

> **Design Philosophy**: Pure Go implementation — no CGO / Rust calls to Lance core, zero external runtime dependencies (beyond OS standard library).

---

## Features

| Capability | Description |
|---|---|
| **Columnar Storage** | Mini-Block columnar encoding with Zstd / LZ4 compression |
| **Vector Search** | Full-scan brute force + IVF-Flat ANN index for approximate nearest neighbor search |
| **Hybrid Query** | Scalar predicate pushdown combined with vector similarity search (dual strategy) |
| **MVCC Versioning** | Snapshot isolation with multi-version concurrency control |
| **Async Batch Writes** | Goroutine + channel buffering with auto-flush and manual flush |
| **Zero-Copy Schema Evolution** | Add / drop columns without rewriting existing data files |
| **Embedded** | No server process — embed as a Go library, like SQLite |
| **Structured Logging** | `log/slog`-based unified logging with operation timing |
| **Unified Error Handling** | API-level error codes (`ErrStorage`, `ErrNotFound`, etc.) + user-friendly messages |

---

## Architecture

```
┌──────────────────────────────────────────────────┐
│                  API Layer                        │
│  Connect / CreateTable / OpenTable / DropTable   │
│  Insert / InsertAsync / Search / Flush / Close   │
│  Query Builder (NewQuery → Vector → Filter→Build)│
│  Structured Error Handling (ErrorCode+UserMessage)│
├──────────────────────────────────────────────────┤
│                  Query Engine                     │
│  BruteForceSearch · ScanFilter (pred. pushdown)  │
│  HybridSearch (FilterFirst / SearchFirst)        │
│  Reranker (multi-source merge)                   │
├──────────────────────────────────────────────────┤
│                  Index Layer                      │
│  Index interface · IVFFlat (K-Means) · Flat      │
├──────────────────────────────────────────────────┤
│              Table / Dataset Layer                │
│  Manifest · Fragment · DataFile · Schema         │
│  FragmentWriter / FragmentReader                  │
│  VersionManager (MVCC cache)                     │
│  ManifestStore (read/write + CAS commit)         │
│  AsyncWriter (channel + goroutine batch write)   │
├──────────────────────────────────────────────────┤
│              Encoding & Compression               │
│  Mini-Block columnar encoding · Zstd · LZ4       │
├──────────────────────────────────────────────────┤
│               Storage Engine                      │
│  ObjectStore interface · LocalFS · BufferPool(LRU)│
│  FileFooter (.lance file trailer)                │
└──────────────────────────────────────────────────┘
```

### Package Dependency Direction

```
api → table ↔ encode ↔ storage
       ↓          ↓
     query ←─── index
         ↓
     distance (shared)
```

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/glancedb/glancedb/api"
    "github.com/glancedb/glancedb/encode"
    "github.com/glancedb/glancedb/table"
)

func main() {
    ctx := context.Background()
    db, err := api.Connect("./golancedb_demo")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // 1. Define schema (id, 4-dim embedding, category)
    schema := table.NewSchema([]*table.Field{
        {Name: "id", Type: encode.TypeInt64, Nullable: false},
        {Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 4},
        {Name: "category", Type: encode.TypeString, Nullable: true},
    })

    tbl, err := db.CreateTable(ctx, "documents", schema)
    if err != nil {
        log.Fatal(err)
    }

    // 2. Insert 10 documents
    for i := 0; i < 10; i++ {
        batch := table.NewRecordBatch(schema, 1)
        batch.SetColumn(0, []int64{int64(i)})
        batch.SetColumn(1, []float32{float32(i), 0, 0, 0})
        category := "science"
        if i%2 == 0 {
            category = "art"
        }
        batch.SetColumn(2, []string{category})
        if err := tbl.Insert(ctx, batch); err != nil {
            log.Fatal(err)
        }
    }
    fmt.Println("Inserted 10 documents.")

    // 3. Vector search: top-3 closest to [3,0,0,0]
    q := api.NewQuery(api.Vector([]float32{3, 0, 0, 0}).Column("embedding")).TopK(3).Build()
    results, _ := tbl.Search(ctx, q)
    for _, r := range results {
        fmt.Printf("  RowID=%d  Score=%.4f\n", r.RowID, r.Score)
    }

    // 4. Hybrid search: only "science" documents closest to [3,0,0,0]
    q2 := api.NewQuery(api.Vector([]float32{3, 0, 0, 0}).Column("embedding")).
        Filter(api.EQ("category", "science")).
        TopK(2).
        Build()
    results2, _ := tbl.Search(ctx, q2)
    for _, r := range results2 {
        fmt.Printf("  RowID=%d  Score=%.4f (category=science)\n", r.RowID, r.Score)
    }

    // 5. Schema evolution: add a column dynamically
    field := &table.Field{Name: "tags", Type: encode.TypeString, Nullable: true}
    tbl.AddColumn(ctx, field)
    fmt.Printf("Schema now has %d fields.\n", tbl.Schema().NumFields())
}
```

See the full example at [examples/quickstart/main.go](examples/quickstart/main.go).

---

## Roadmap

### ✅ Phase 1 — Core Storage Engine

- [x] JSON serialization for Manifest & Fragment
- [x] Mini-Block columnar encoding with Zstd / LZ4 compression
- [x] Zero-copy data evolution (incremental writes, no rewrites)
- [x] Local filesystem I/O with Buffer Pool (LRU cache)

### ✅ Phase 2 — Concurrency & Memory Management

- [x] MVCC version manager (in-memory version tracking with auto-eviction)
- [x] Concurrent read/write with `sync.RWMutex`
- [x] Async batch writes via goroutines + channels (AsyncWriter)
- [x] Optimistic lock CAS commit protocol (ManifestStore.Commit)
- [x] Manifest version management (ListVersions / DeleteVersion)

### ✅ Phase 3 — Query Engine & Indexing

- [x] Brute-force vector search (cosine / euclidean / dot product)
- [x] IVF-Flat ANN index (pure Go K-Means clustering, nProbes support)
- [x] Scalar predicate pushdown (EQ / NE / LT / GT / LE / GE / In)
- [x] Hybrid search dual strategy (FilterFirst / SearchFirst)
- [x] Result reranking (Reranker)

### ✅ Phase 4 — API & MVP

- [x] Developer-friendly API: `Connect` / `CreateTable` / `OpenTable` / `DropTable`
- [x] Sync `Insert` + async `InsertAsync` / `Flush`
- [x] Query builder: `NewQuery` → `Vector` → `Filter` → `Build`
- [x] Unified error types (`ErrorCode` + `UserMessage` + `wrapStorageErr`)
- [x] Structured logging (`log/slog` with operation timing)
- [x] End-to-end test coverage: insert, search, hybrid filter, async write, schema evolution, storage failure
- [x] Schema evolution (AddColumn / DropColumn)

### 🗓️ Phase 5 — Performance Benchmarks (Pending)

- [ ] `go test -bench` benchmark suite
- [ ] `pprof` CPU / memory profiling
- [ ] Comparison benchmarks vs Lance (Rust)
- [ ] Large-scale stress testing

---

## Dependencies

| Dependency | Purpose | License |
|---|---|---|
| `github.com/klauspost/compress` | Zstd compression | Apache-2.0 + BSD-3-Clause |
| `github.com/pierrec/lz4/v4` | LZ4 compression | BSD-3-Clause |

**Zero CGO. Zero external database dependencies.** Manifest serialization uses stdlib `encoding/json`.

---

## Platform Support

| Platform | Status |
|---|---|
| Linux (amd64 / arm64) | Primary target, CI-verified |
| macOS (amd64 / arm64) | Development environment |
| Windows (amd64) | Compatible (cross-platform paths with `filepath`) |

**Go version requirement**: Go 1.22+

---

## Project Structure

```
glancedb/
├── api/            # Public-facing API (Database, Table, QueryBuilder, Errors, Logger)
│   ├── connection.go       # Database management (Connect / Close / CreateTable / OpenTable)
│   ├── table.go            # Table wrapper (Insert / InsertAsync / Search / Flush)
│   ├── query.go            # Query builder (NewQuery → Vector → Filter → Build)
│   ├── errors.go           # Unified error types (ErrorCode + UserMessage)
│   ├── logger.go           # Structured logging (slog wrapper)
│   └── api_e2e_test.go     # End-to-end integration tests
├── table/           # Table / Dataset layer
│   ├── manifest.go         # Manifest + Schema serialization (JSON)
│   ├── manifest_store.go   # Manifest read/write + CAS commit + version management
│   ├── fragment.go         # Fragment / DataFile structures
│   ├── fragment_writer.go  # Fragment writing + RecordBatch
│   ├── fragment_reader.go  # Fragment reading (column decode)
│   ├── schema.go           # Field / Schema definitions
│   ├── version_manager.go  # MVCC version cache
│   └── async_writer.go     # Async batch writer
├── encode/          # Columnar encoding
│   ├── interface.go        # ColumnEncoder interface
│   ├── miniblock.go        # Mini-Block encode / decode
│   ├── constant.go         # Constant layout
│   └── compression.go      # Zstd / LZ4 compression
├── storage/         # Storage engine
│   ├── object_store.go     # ObjectStore interface
│   ├── local_fs.go         # Local filesystem implementation
│   ├── buffer_pool.go      # Buffer Pool (LRU cache)
│   └── file_footer.go      # .lance file trailer
├── distance/        # Shared distance computation
│   ├── types.go            # DistanceMetric, SearchResult
│   ├── distance.go         # Distance / Distances / TopK functions
│   └── distance_test.go    # 13 test cases
├── query/           # Query engine
│   ├── types.go            # Query, VectorQuery, ScalarFilter definitions
│   ├── brute_force.go      # Brute-force vector search
│   ├── scan_filter.go      # Scalar filtering (predicate pushdown)
│   ├── hybrid_search.go    # Hybrid search (dual strategy)
│   └── reranker.go         # Result reranking
├── index/           # Index system
│   ├── interface.go        # Index interface
│   ├── ivf_flat.go         # IVF + Flat ANN index
│   ├── flat.go             # Flat baseline index
│   ├── types.go            # Type definitions
│   └── ivf_flat_test.go    # 12 test cases
└── examples/        # Usage examples
    └── quickstart/
        └── main.go         # Complete example application
```

---

## Development Commands

```bash
# Run all tests
go test ./...

# With race detection
go test -race ./...

# Run tests for a specific package
go test -v ./api/

# Format check
go fmt ./...

# Static analysis
go vet ./...
```

---

## Design

See the [design document](design/design.md) for detailed architecture, core data structures, component relationships, and design decisions.

---

## License

Apache 2.0
