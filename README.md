# GolanceDB

**A Go-based vector database engine built on the Lance columnar format.**

GolanceDB is an open-source, embedded vector database engine implemented in pure Go, designed for AI/ML workloads. It provides columnar storage, vector similarity search, scalar filtering, and MVCC versioning — all without external dependencies or CGO.

> **Design Philosophy**: Pure Go implementation — no Rust/CGO calls to the Lance core library, zero external runtime dependencies (beyond OS libraries).

---

## Features

| Capability | Description |
|---|---|
| **Columnar Storage** | Lance-format columnar encoding (Mini-Block) with Zstd/LZ4 compression |
| **Vector Search** | Brute-force + IVF-Flat ANN index for approximate nearest neighbor search |
| **Hybrid Query** | Scalar predicate pushdown combined with vector similarity search |
| **MVCC Versioning** | Snapshot isolation with time-travel queries |
| **Zero-Copy Evolution** | Add/drop/rename columns without rewriting existing data files |
| **Embedded** | No server process — embed as a Go library, like SQLite |
| **Pluggable Storage** | Local filesystem (Phase 1), object storage S3/GCS (future) |

---

## Architecture

```
┌──────────────────────────────────────────────────┐
│                   API Layer                       │
│  Connect / CreateTable / Insert / Search / Delete │
├──────────────────────────────────────────────────┤
│                  Query Engine                     │
│  Vector Search │ Scalar Filter │ Hybrid Planner   │
├──────────────────────────────────────────────────┤
│                  Index Layer                      │
│          IVFFlat · HNSW (future)                  │
├──────────────────────────────────────────────────┤
│              Table / Dataset Layer                │
│    Manifest · Fragment · DataFile · DeletionFile  │
├──────────────────────────────────────────────────┤
│               Storage Engine                      │
│   Page Manager · Block Compress · Encoding/Decode │
│                  Buffer Pool (LRU)                │
├──────────────────────────────────────────────────┤
│                  I/O Layer                        │
│       Local File (os.File) · ObjectStore (future) │
└──────────────────────────────────────────────────┘
```

---

## Roadmap

### Phase 1 — Core Storage Engine

- [ ] Protobuf protocol for Manifest & Fragment serialization
- [ ] Mini-Block columnar encoding with Zstd compression
- [ ] Zero-copy data evolution (incremental writes, no rewrites)
- [ ] Local filesystem I/O with Buffer Pool (LRU cache)

### Phase 2 — Concurrency & Memory Management

- [ ] MVCC version manager (in-memory version tracking)
- [ ] Concurrent read/write with `sync.RWMutex`
- [ ] Async batched writes via goroutines + channels
- [ ] Optimistic lock commit protocol with CAS

### Phase 3 — Query Engine & Indexing

- [ ] Brute-force vector search (cosine, euclidean, dot product)
- [ ] IVF-Flat ANN index (K-means clustering in pure Go)
- [ ] Scalar predicate pushdown (column-level filtering)
- [ ] Hybrid search (filter-then-search / search-then-filter)

### Phase 4 — API & MVP

- [ ] Developer-friendly API: `Connect`, `CreateTable`, `Insert`, `Search`
- [ ] Structured error handling and logging (`log/slog`)
- [ ] Performance benchmarks (`go test -bench` + `pprof`)
- [ ] End-to-end example application

---

## Quick Start (Coming Soon)

```go
import "github.com/glancedb/glancedb"

func main() {
    ctx := context.Background()
    db, _ := glancedb.Connect(ctx, "./data", nil)
    defer db.Close()

    // Define schema
    schema := &glancedb.Schema{
        Fields: []*glancedb.Field{
            {Name: "id", Type: glancedb.TypeInt64},
            {Name: "embedding", Type: glancedb.TypeFixedSizeList, 
             Metadata: map[string]string{"dim": "128"}},
            {Name: "text", Type: glancedb.TypeString},
        },
    }

    // Create table
    table, _ := db.CreateTable(ctx, "documents", schema, nil)

    // Insert data
    table.Insert(ctx, &glancedb.RecordBatch{...})

    // Search
    results, _ := table.Search(ctx, &glancedb.Query{
        Vector: &glancedb.VectorQuery{
            Vector: []float32{...},
            Column: "embedding",
            Metric: glancedb.DistanceCosine,
        },
        Limit: 10,
    })
}
```

---

## Dependencies

| Dependency | Purpose | License |
|---|---|---|
| `google.golang.org/protobuf` | Manifest/metadata serialization | BSD |
| `github.com/RoaringBitmap/roaring` | Deletion bitmap | Apache-2.0 |
| `github.com/klauspost/compress` | Zstd / LZ4 compression | Apache-2.0 + BSD-3-Clause |

**Zero CGO. Zero external database dependencies.**

---

## Platform Support

| Platform | Status |
|---|---|
| Linux (amd64 / arm64) | Primary target |
| macOS (amd64 / arm64) | Development target |
| Windows (amd64) | Compatible (no mmap for now) |

**Go version requirement**: Go 1.22+

---

## Project Structure

```
glancedb/
├── api/          # Public-facing API (Connection, Table, Query)
├── table/        # Table/Dataset layer (Manifest, Fragment, Schema)
├── encode/       # Columnar encoding (Mini-Block, compression)
├── storage/      # Storage engine (ObjectStore, BufferPool)
├── query/        # Query engine (vector search, scalar filter)
├── index/        # Index system (IVFFlat, distance metrics)
├── proto/        # Protobuf definitions
├── benchmark/    # Performance benchmarks
└── examples/     # Usage examples
```

---

## Design

See the [design document](design/design.md) for detailed architecture, core data structures, component relationships, and design decisions.

---

## License

Apache 2.0
