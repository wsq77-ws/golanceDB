# GlanceDB — AI Agent Guide

## 项目背景

GlanceDB 是一个纯 Go 实现的嵌入式向量数据库引擎，兼容 Lance 列式存储格式。目标是为 AI/ML 场景提供高性能的向量存储、检索和数据管理能力。

**核心设计原则**：
- 纯 Go 实现，不依赖 CGO / Rust 调用
- 零外部运行时依赖（除 OS 标准库外）
- 嵌入式部署，无独立服务进程
- 与 Lance 格式兼容（Mini-Block 编码、列式存储布局）

**关键文档**（`/design/` 目录由 `.gitignore` 排除，AI 仍可直接读取）：
- [design.md](design/design.md) — 完整架构设计、数据结构、接口定义
- [roadmap.md](design/roadmap.md) — 四阶段开发路线图

---

## 外部依赖

| 依赖 | 用途 | 何时需要 |
|---|---|---|
| `github.com/klauspost/compress/zstd` | Zstd 压缩（列数据写入） | 数据写入时压缩 |
| `github.com/pierrec/lz4/v4` | LZ4 压缩（可选） | 需要 LZ4 压缩的场景 |

**说明**：
- Manifest 元数据使用标准库 `encoding/json`，无需 protobuf 依赖
- 删除标记功能开发前无需引入 `RoaringBitmap`
- ManifestStore 通过 `storage.Store` 接口操作，支持本地文件系统和未来 S3/FUSE

**禁止引入**：
- 禁止 CGO 调用外部库（如 Lance Rust 核心）
- 禁止依赖外部数据库（如 PostgreSQL、etcd）
- 禁止引入重量级框架

---

## 目录结构

```
glancedb/
├── config/       # 配置系统
│   └── config.go           # Config 结构体、JSON Load/Save、Store 工厂、Validate
├── api/          # 对外暴露的 API（Database, Table, Query, Errors, Logger）
│   ├── connection.go       # Database 管理（Connect / ConnectWithConfig / Close）
│   ├── table.go            # Table 封装（Insert / BatchInsert / InsertAsync / Search / Flush）
│   ├── query.go            # 查询构建器（Vector → Filter → Build）
│   ├── errors.go           # 统一错误类型（ErrorCode + UserMessage）
│   └── logger.go           # 结构化日志（slog 封装）
├── table/        # 表 / Dataset 层
│   ├── manifest.go         # Manifest 结构 + JSON 序列化
│   ├── manifest_store.go   # Manifest 读写 + 版本管理（基于 Store 接口）
│   ├── fragment.go         # Fragment / DataFile / DeletionFile 结构
│   ├── fragment_writer.go  # Fragment 写入 + RecordBatch
│   ├── fragment_reader.go  # Fragment 读取（列解码）
│   ├── schema.go           # Schema / Field 定义
│   ├── version_manager.go  # MVCC 版本管理（LRU 缓存）
│   └── async_writer.go     # 异步批量写入（goroutine + channel）
├── encode/       # 列式编码
│   ├── interface.go        # ColumnEncoder 接口
│   ├── miniblock.go        # Mini-Block 编码/解码
│   ├── constant.go         # Constant 布局
│   └── compression.go      # 压缩/解压（Zstd, LZ4）
├── storage/      # 存储引擎
│   ├── object_store.go     # Store 统一接口 + ObjectStore 类型别名
│   ├── local_fs.go         # 本地文件系统实现
│   ├── buffer_pool.go      # Buffer Pool（LRU 缓存）
│   └── file_footer.go      # File Footer 读写（40 字节）
├── distance/     # 公共距离计算
│   ├── types.go            # DistanceMetric, SearchResult
│   ├── distance.go         # Distance, Distances, TopK 函数
│   └── distance_test.go    # 13 个测试用例
├── query/        # 查询引擎
│   ├── types.go            # Query / VectorQuery / ScalarFilter 定义
│   ├── brute_force.go      # 暴力向量搜索
│   ├── scan_filter.go      # 标量过滤（谓词下推）
│   ├── hybrid_search.go    # 混合搜索（双策略）
│   └── reranker.go         # 结果重排序
├── index/        # 索引系统
│   ├── interface.go        # Index 接口定义
│   ├── types.go            # 类型定义（VectorRecord / IndexType / IndexStats）
│   ├── ivf_flat.go         # IVF + Flat ANN 索引（K-Means 聚类）
│   └── flat.go             # 暴力基线索引
└── examples/     # 使用示例
    └── quickstart/         # 完整使用示例
```

---

## 开发规范

### 1. 单元测试

- 每个包必须有对应的 `_test.go` 文件，测试覆盖核心路径和边界条件。
- 新功能提交前必须执行：`go test -race ./...`（要求竞态检测通过）
- 使用 Go 原生 `testing` 包，不需要第三方测试框架。
- Benchmark 写在 `benchmark/` 目录下（阶段五），使用 `go test -bench=. -benchmem`。

**测试要求**：
- **正常路径**：验证功能正确性（如 Insert 后能 Search 到）
- **边界条件**：空输入、nil 指针、零长度向量、单行/超多行数据
- **错误路径**：文件不存在、权限不足、版本冲突、Schema 不匹配、存储故障恢复
- **并发安全**：使用 `go test -race` 验证无数据竞争
- **跨平台**：涉及文件路径操作时，使用 `filepath.Join` 而非字符串拼接
- **存储抽象**：所有测试通过 `storage.Store` 接口操作，不依赖具体后端

### 2. 错误处理

- 所有可能失败的函数必须返回 `error`，禁止 panic（除初始化阶段的 fatal 错误外）。
- 使用 `fmt.Errorf("package: %w", err)` 包装错误，保留错误链。
- 在 `api/errors.go` 中定义公共错误码，提供 `UserMessage()` 返回用户友好提示。
- 使用 `wrapStorageErr()` 将所有底层错误统一为 `api.Error`，确保存储故障时既有结构化日志又有用户友好信息。
- 错误信息应包含足够的上下文（如文件名、版本号、列 ID、操作名）。

### 3. 代码风格

- 遵循 `go fmt` 格式，commit 前运行 `go fmt ./...`
- 遵循 `go vet` 静态检查，commit 前运行 `go vet ./...`
- 命名规范：
  - 包名小写单数（`table` 而非 `tables`）
  - 接口名以 `er` 结尾（`Encoder`, `Store`）
  - 导出类型/函数使用驼峰（`CreateTable`, `NewFragmentWriter`）
  - 私有字段使用驼峰（`fieldID`, `numRows`）
- 禁止使用 `init()` 函数（除非绝对必要）
- 禁止使用 `context.Background()` 在生产路径中（仅可在外层入口如 `Connect` 中用于日志兜底）

### 4. 注释规范

- **注释要精简**。优先用好的命名代替注释。
- 导出类型和函数必须有 godoc 注释，格式：`// FunctionName does X.`（句号结尾）
- 实现细节注释只写在逻辑不直观的地方。
- 不要写"显而易见"的注释（如 `// increment i`）。
- TODO 注释格式：`// TODO(username): what to do`

### 5. 跨平台

- 所有文件路径操作使用 `path/filepath` 包，禁止硬编码 `/` 或 `\`。
- 禁止使用 `syscall` / `golang.org/x/sys` 的非可移植接口。
- 行尾使用 LF（Go 工具链默认处理）。
- 涉及 mmap 等平台特定功能时，使用 `_linux.go`, `_windows.go` 构建标签。
- CI 至少在 Linux amd64 + Windows amd64 上运行测试。
- 测试用的临时文件应使用 `t.TempDir()` 自动清理，禁止手动创建测试目录。

---

## 可扩展性指南

GlanceDB 的设计目标之一是从 MVP 平滑演进到生产级系统。以下原则确保未来扩展时不需大规模重构：

### 6.1 接口先行

- 存储后端通过 `storage.Store` 统一接口抽象。`LocalFS` 实现时可确保接口设计能支持 S3 / FUSE / GCS。
- 配置系统通过 `config.Config.NewStore()` 工厂方法选择后端，新增后端只需添加 `BackendType` + 分支。
- 编码器通过 `ColumnEncoder` 接口抽象。未来新增编码格式只需实现该接口。
- 索引通过 `Index` 接口抽象。IVFFlat 和未来的 HNSW 都实现同一接口。

### 6.2 不引入无用抽象

- 不要为只有一个实现的接口创建单独的接口文件。等到出现第二种实现时再提取接口。
- 不要创建 `types.go` 或 `constants.go` 这样的杂项文件。类型定义放在最相关的文件里。
- 不要为了"未来的可能性"预留钩子和回调。YAGNI（You Ain't Gonna Need It）。

### 6.3 包依赖方向

```
config (纯数据，不依赖项目内其他包)
       ↓
api → table ↔ encode ↔ storage
       ↓          ↓
     query ←─── index
```

- `config` 包是纯数据定义，不导入任何项目内部包（仅导入 `storage` 用于工厂函数）。
- `table` 包可以依赖 `encode` 和 `storage`。
- `query` 包可以依赖 `table` 和 `index`。
- `api` 包依赖所有下层包。
- **禁止循环依赖**。如果出现循环引用，说明分层有问题。

### 6.4 版本兼容

- Manifest 使用 JSON 序列化（stdlib `encoding/json`），新增字段应使用指针或带 `omitempty` 标签。
- 禁止删除或重命名已有的 JSON 字段名，优先使用 `json:"-"` 忽略旧字段。
- 数据文件的 FileFooter 中 `MajorVersion` / `MinorVersion` 用于向后兼容。
- ManifestStore 的 `_latest` 指针文件内容为版本号字符串，向前兼容（新版本号总是递增）。

---

## 安全与提交规范

### 7.1 禁止提交到 GitHub

- **构建产物**：二进制文件、`/tmp/` 输出
- **IDE/编辑器配置**：`.idea/`, `.vscode/`
- **敏感信息**：密码、token、证书、私钥、`.env` 文件
- **覆盖报告**：`coverage.txt`, `coverage.out`
- **设计文档**：`/design/` 目录不提交

> 参见 `.gitignore` 获取完整列表。

### 7.2 Commit 规范

- 使用 `git status` + `git diff` 确认变更内容后再 commit。
- Commit message 格式：`<package>: <简短描述>`（如 `storage: add BufferPool LRU eviction`）
- 禁止 commit 调试日志、注释掉的代码段、未完成的 WIP 代码。
- 不要将 `/design/` 目录下的设计文档提交到主分支。

### 7.3 README 维护

- 新增公共 API 后，更新 `README.md` 和 `README_CN.md` 中的 Quick Start 示例。
- 外部依赖发生变化时，更新两个 README 的依赖表。
- 目录结构变化时，同步更新两个 README 的项目结构图。

---

## 常用命令

```bash
# 运行所有测试（含竞态检测）
go test -race ./...

# 运行特定包测试
go test -v ./api/
go test -v ./table/

# 运行 Benchmark
go test -bench=. -benchmem ./benchmark/

# 检查代码格式
go fmt ./...

# 静态分析
go vet ./...

# 构建
go build ./...
```

---

## 查看设计文档

完整架构设计请参考 [design/design.md](design/design.md)，包含：
- 分层架构与组件职责
- 所有核心数据结构的 Go 定义
- 主要接口签名
- 读写流程与组件关系图
- 物理磁盘存储布局
- 四阶段开发规划
