# GolanceDB

**基于 Lance 列式格式的 Go 语言嵌入式向量数据库引擎**

GolanceDB 是一个纯 Go 实现的嵌入式向量数据库引擎，兼容 Lance 列式存储格式。专为 AI/ML 场景设计，提供列式存储、向量相似度搜索、标量过滤和 MVCC 版本控制 — 零 CGO 依赖，零外部运行时依赖。

> **设计原则**：纯 Go 实现，不依赖 CGO / Rust 调用，零外部运行时依赖（除操作系统标准库外）。

---

## 核心能力

| 能力 | 说明 |
|---|---|
| **列式存储** | 基于 Lance 格式的 Mini-Block 列式编码，支持 Zstd / LZ4 压缩 |
| **向量检索** | 暴力全表扫描 + IVF-Flat ANN 索引，支持近似最近邻搜索 |
| **混合查询** | 标量谓词下推 + 向量相似度联合搜索（双策略：先过滤 / 先搜索） |
| **MVCC 版本控制** | 快照隔离，支持多版本并发控制 |
| **异步批量写入** | Goroutine + Channel 批量合并写入，定时自动刷盘 |
| **零拷贝 Schema 演进** | 添加/删除列无需重写已有数据文件 |
| **嵌入式部署** | 无独立服务进程，像 SQLite 一样作为 Go 库集成 |
| **结构化日志** | 基于 `log/slog` 的统一日志，操作耗时追踪 |
| **统一错误处理** | API 层统一错误码 + 用户友好错误信息 |

---

## 架构

```
┌──────────────────────────────────────────────────┐
│                  API 层                           │
│  Connect / CreateTable / OpenTable / DropTable   │
│  Insert / InsertAsync / Search / Flush / Close   │
│  查询构建器 (NewQuery → Vector → Filter → Build)  │
│  结构化错误处理 (ErrorCode + UserMessage)         │
├──────────────────────────────────────────────────┤
│                 查询引擎                          │
│  BruteForceSearch · ScanFilter (谓词下推)         │
│  HybridSearch (FilterFirst / SearchFirst)        │
│  Reranker (多源结果合并)                          │
├──────────────────────────────────────────────────┤
│                 索引层                            │
│  Index 接口 · IVFFlat (K-Means 聚类) · Flat      │
├──────────────────────────────────────────────────┤
│              表 / Dataset 层                     │
│  Manifest · Fragment · DataFile · Schema         │
│  FragmentWriter / FragmentReader                  │
│  VersionManager (MVCC 缓存)                      │
│  ManifestStore (版本读写 + CAS 提交)             │
│  AsyncWriter (Channel + Goroutine 批量写入)      │
├──────────────────────────────────────────────────┤
│               编码 & 压缩                         │
│  Mini-Block 列式编码 · Zstd · LZ4               │
├──────────────────────────────────────────────────┤
│               存储引擎                            │
│  Store 接口 · LocalFS · BufferPool (LRU)        │
│  FileFooter (.lance 文件尾)                      │
└──────────────────────────────────────────────────┘
```

### 包依赖方向

```
api → table ↔ encode ↔ storage
       ↓          ↓
     query ←─── index
         ↓
     distance (公共)
```

---

## 快速开始

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

    // 1. 定义 Schema（id, 4维向量, 分类标签）
    schema := table.NewSchema([]*table.Field{
        {Name: "id", Type: encode.TypeInt64, Nullable: false},
        {Name: "embedding", Type: encode.TypeFixedSizeList, Dimension: 4},
        {Name: "category", Type: encode.TypeString, Nullable: true},
    })

    tbl, err := db.CreateTable(ctx, "documents", schema)
    if err != nil {
        log.Fatal(err)
    }

    // 2. 插入 10 条数据
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

    // 3. 向量搜索：查找离 [3,0,0,0] 最近的 3 条记录
    q := api.NewQuery(api.Vector([]float32{3, 0, 0, 0}).Column("embedding")).TopK(3).Build()
    results, _ := tbl.Search(ctx, q)
    for _, r := range results {
        fmt.Printf("  RowID=%d  Score=%.4f\n", r.RowID, r.Score)
    }

    // 4. 混合搜索：仅搜索 "science" 分类
    q2 := api.NewQuery(api.Vector([]float32{3, 0, 0, 0}).Column("embedding")).
        Filter(api.EQ("category", "science")).
        TopK(2).
        Build()
    results2, _ := tbl.Search(ctx, q2)
    for _, r := range results2 {
        fmt.Printf("  RowID=%d  Score=%.4f (category=science)\n", r.RowID, r.Score)
    }

    // 5. Schema 演进：动态添加列
    field := &table.Field{Name: "tags", Type: encode.TypeString, Nullable: true}
    tbl.AddColumn(ctx, field)
    fmt.Printf("Schema now has %d fields.\n", tbl.Schema().NumFields())
}
```

完整示例见 [examples/quickstart/main.go](examples/quickstart/main.go)。

---

## 开发路线图

### ✅ 阶段一 — 核心存储引擎

- [x] Manifest & Fragment 的 JSON 序列化协议
- [x] Mini-Block 列式编码 + Zstd / LZ4 压缩
- [x] 零拷贝数据演进（增量写入，不改写历史文件）
- [x] 本地文件系统 I/O + Buffer Pool（LRU 缓存）

### ✅ 阶段二 — 并发与内存管理

- [x] MVCC 版本管理器（内存版本追踪 + 自动淘汰）
- [x] `sync.RWMutex` 并发读写安全
- [x] Goroutine + Channel 异步批量写入（AsyncWriter）
- [x] 乐观锁 CAS 提交协议（ManifestStore.Commit）
- [x] Manifest 版本文件管理（ListVersions / DeleteVersion）

### ✅ 阶段三 — 检索引擎与索引

- [x] 暴力向量搜索（余弦距离 / 欧氏距离 / 点积）
- [x] IVF-Flat ANN 索引（纯 Go K-Means 聚类，支持 nProbes）
- [x] 标量谓词下推（EQ / NE / LT / GT / LE / GE / In）
- [x] 混合搜索双策略（FilterFirst / SearchFirst）
- [x] 结果重排序（Reranker）

### ✅ 阶段四 — API 与 MVP 验证

- [x] 开发者友好 API：`Connect` / `CreateTable` / `OpenTable` / `DropTable`
- [x] 同步写入 `Insert` + 异步写入 `InsertAsync` / `Flush`
- [x] 查询构建器：`NewQuery` → `Vector` → `Filter` → `Build`
- [x] 统一错误类型（`ErrorCode` + `UserMessage` + `wrapStorageErr`）
- [x] 结构化日志（基于 `log/slog`，操作耗时追踪）
- [x] 端到端测试覆盖：插入、搜索、混合过滤、异步写入、Schema 演进、存储故障
- [x] Schema 演进（AddColumn / DropColumn）

### 🗓️ 阶段五 — 性能基准测试（待完成）

- [ ] `go test -bench` Benchmark 套件
- [ ] `pprof` CPU / 内存性能分析
- [ ] 与 Lance (Rust) 基准对比
- [ ] 大数据集压力测试

---

## 外部依赖

| 依赖 | 用途 | 许可 |
|---|---|---|
| `github.com/klauspost/compress` | Zstd 压缩 | Apache-2.0 + BSD-3-Clause |
| `github.com/pierrec/lz4/v4` | LZ4 压缩 | BSD-3-Clause |

**零 CGO 依赖。零外部数据库依赖。** Manifest 使用标准库 `encoding/json` 序列化。

---

## 平台支持

| 平台 | 状态 |
|---|---|
| Linux (amd64 / arm64) | 主要目标，CI 已验证 |
| macOS (amd64 / arm64) | 开发环境 |
| Windows (amd64) | 兼容（`filepath` 跨平台路径） |

**Go 版本要求**：Go 1.22+

---

## 目录结构

```
glancedb/
├── api/            # 对外 API（Database, Table, QueryBuilder, Errors, Logger）
│   ├── connection.go       # Database 管理（Connect / Close / CreateTable / OpenTable）
│   ├── table.go            # Table 封装（Insert / InsertAsync / Search / Flush）
│   ├── query.go            # 查询构建器（NewQuery → Vector → Filter → Build）
│   ├── errors.go           # 统一错误类型（ErrorCode + UserMessage）
│   ├── logger.go           # 结构化日志（slog 包装）
│   └── api_e2e_test.go     # 端到端集成测试
├── table/           # 表 / Dataset 层
│   ├── manifest.go         # Manifest + Schema 序列化（JSON）
│   ├── manifest_store.go   # Manifest 读写 + CAS 提交 + 版本管理
│   ├── fragment.go         # Fragment / DataFile 结构
│   ├── fragment_writer.go  # Fragment 写入 + RecordBatch
│   ├── fragment_reader.go  # Fragment 读取（列解码）
│   ├── schema.go           # Field / Schema 定义
│   ├── version_manager.go  # MVCC 版本缓存
│   └── async_writer.go     # 异步批量写入器
├── encode/          # 列式编码
│   ├── interface.go        # ColumnEncoder 接口
│   ├── miniblock.go        # Mini-Block 编码 / 解码
│   ├── constant.go         # Constant 布局
│   └── compression.go      # Zstd / LZ4 压缩
├── storage/         # 存储引擎
│   ├── object_store.go     # Store 接口
│   ├── local_fs.go         # 本地文件系统实现
│   ├── buffer_pool.go      # Buffer Pool（LRU 缓存）
│   └── file_footer.go      # .lance 文件尾
├── distance/        # 公共距离计算
│   ├── types.go            # DistanceMetric, SearchResult
│   ├── distance.go         # Distance / Distances / TopK 函数
│   └── distance_test.go    # 13 个测试用例
├── query/           # 查询引擎
│   ├── types.go            # Query, VectorQuery, ScalarFilter 定义
│   ├── brute_force.go      # 暴力向量搜索
│   ├── scan_filter.go      # 标量过滤（谓词下推）
│   ├── hybrid_search.go    # 混合搜索（双策略）
│   └── reranker.go         # 结果重排序
├── index/           # 索引系统
│   ├── interface.go        # Index 接口定义
│   ├── ivf_flat.go         # IVF + Flat ANN 索引
│   ├── flat.go             # 暴力基线索引
│   ├── types.go            # 类型定义
│   └── ivf_flat_test.go    # 12 个测试用例
└── examples/        # 使用示例
    └── quickstart/
        └── main.go         # 完整示例应用
```

---

## 开发命令

```bash
# 运行所有测试
go test ./...

# 带竞态检测
go test -race ./...

# 运行指定包的测试
go test -v ./api/

# 格式检查
go fmt ./...

# 静态分析
go vet ./...
```

---

## 设计文档

详细的架构设计、核心数据结构、组件关系和设计决策请参考[设计文档](design/design.md)。

---

## 许可协议

Apache 2.0
