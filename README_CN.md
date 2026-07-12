# GolanceDB

**基于 Lance 列式格式的 Go 语言向量数据库引擎**

GolanceDB 是一个纯 Go 实现的嵌入式向量数据库引擎，专为 AI/ML 场景设计。提供列式存储、向量相似度搜索、标量过滤和 MVCC 版本控制能力 — 零外部依赖，无需 CGO。

> **设计原则**：纯 Go 实现，不依赖 Rust/CGO 调用 Lance 核心库，保持零外部运行时依赖（除操作系统库外）。

---

## 核心能力

| 能力 | 说明 |
|---|---|
| **列式存储** | 基于 Lance 格式的 Mini-Block 列式编码，支持 Zstd/LZ4 压缩 |
| **向量检索** | 暴力搜索 + IVF-Flat ANN 索引，支持近似最近邻搜索 |
| **混合查询** | 标量谓词下推 + 向量相似度联合搜索 |
| **MVCC 版本控制** | 快照隔离，支持时间旅行查询（查看/回滚历史版本） |
| **零拷贝演进** | 添加/删除/重命名列无需重写已有数据文件 |
| **嵌入式部署** | 无独立服务进程，像 SQLite 一样作为 Go 库集成 |
| **可插拔存储** | 本地文件系统（Phase 1），后续支持 S3/GCS 对象存储 |

---

## 架构

```
┌──────────────────────────────────────────────────┐
│                  API 层                           │
│  Connect / CreateTable / Insert / Search / Delete │
├──────────────────────────────────────────────────┤
│                 查询引擎                          │
│  向量搜索 │ 标量过滤 │ 混合查询规划器             │
├──────────────────────────────────────────────────┤
│                 索引层                            │
│          IVFFlat · HNSW（未来）                    │
├──────────────────────────────────────────────────┤
│              表 / Dataset 层                     │
│    Manifest · Fragment · DataFile · DeletionFile │
├──────────────────────────────────────────────────┤
│               存储引擎                            │
│  页面管理 · 块压缩 · 编码/解码 · Buffer Pool     │
├──────────────────────────────────────────────────┤
│                I/O 层                            │
│       本地文件 (os.File) · 对象存储 (未来)        │
└──────────────────────────────────────────────────┘
```

---

## 开发路线图

### 阶段一 — 核心存储引擎

- [ ] Manifest & Fragment 的 Protobuf 序列化协议
- [ ] Mini-Block 列式编码 + Zstd 压缩
- [ ] 零拷贝数据演进（增量写入，不改写历史文件）
- [ ] 本地文件系统 I/O + Buffer Pool（LRU 缓存）

### 阶段二 — 并发与内存管理

- [ ] MVCC 版本管理器（内存版本追踪）
- [ ] `sync.RWMutex` 并发读写安全
- [ ] Goroutine + Channel 异步批量写入
- [ ] 乐观锁 CAS 提交协议

### 阶段三 — 检索引擎与索引

- [ ] 暴力向量搜索（余弦距离、欧氏距离、点积）
- [ ] IVF-Flat ANN 索引（纯 Go K-Means 聚类）
- [ ] 标量谓词下推（列级过滤）
- [ ] 混合搜索（先过滤再搜索 / 先搜索再过滤）

### 阶段四 — API 与 MVP 验证

- [ ] 开发者友好 API：`Connect`、`CreateTable`、`Insert`、`Search`
- [ ] 结构化错误处理与日志（`log/slog`）
- [ ] 性能基准测试（`go test -bench` + `pprof`）
- [ ] 端到端示例应用

---

## 快速开始（即将推出）

```go
import "github.com/glancedb/glancedb"

func main() {
    ctx := context.Background()
    db, _ := glancedb.Connect(ctx, "./data", nil)
    defer db.Close()

    // 定义 Schema
    schema := &glancedb.Schema{
        Fields: []*glancedb.Field{
            {Name: "id", Type: glancedb.TypeInt64},
            {Name: "embedding", Type: glancedb.TypeFixedSizeList,
             Metadata: map[string]string{"dim": "128"}},
            {Name: "text", Type: glancedb.TypeString},
        },
    }

    // 创建表
    table, _ := db.CreateTable(ctx, "documents", schema, nil)

    // 插入数据
    table.Insert(ctx, &glancedb.RecordBatch{...})

    // 向量搜索
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

## 外部依赖

| 依赖 | 用途 | 许可 |
|---|---|---|
| `google.golang.org/protobuf` | Manifest/元数据序列化 | BSD |
| `github.com/RoaringBitmap/roaring` | 删除标记位图 | Apache-2.0 |
| `github.com/klauspost/compress` | Zstd / LZ4 压缩 | Apache-2.0 + BSD-3-Clause |

**零 CGO 依赖。零外部数据库依赖。**

---

## 平台支持

| 平台 | 状态 |
|---|---|
| Linux (amd64 / arm64) | 主要目标 |
| macOS (amd64 / arm64) | 开发环境 |
| Windows (amd64) | 兼容（暂时不支持 mmap） |

**Go 版本要求**：Go 1.22+

---

## 目录结构

```
glancedb/
├── api/          # 对外 API（Connection, Table, Query）
├── table/        # 表/Dataset 层（Manifest, Fragment, Schema）
├── encode/       # 列式编码（Mini-Block, 压缩）
├── storage/      # 存储引擎（ObjectStore, BufferPool）
├── query/        # 查询引擎（向量搜索, 标量过滤）
├── index/        # 索引系统（IVFFlat, 距离计算）
├── proto/        # Protobuf 协议定义
├── benchmark/    # 性能基准测试
└── examples/     # 使用示例
```

---

## 设计文档

详细的架构设计、核心数据结构、组件关系以及设计决策请参考[设计文档](design/design.md)。

---

## 许可协议

Apache 2.0
