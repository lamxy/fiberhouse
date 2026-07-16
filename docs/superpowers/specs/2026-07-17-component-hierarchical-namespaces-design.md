# Component 分层命名空间重组设计

**日期：** 2026-07-17

**状态：** 已完成交互设计确认，待书面规格审阅

## 背景

FiberHouse 已将 `component/` 定义为内置、可选装配、可复用能力的命名空间，并完成 container、cache 与 database 的首轮归类。当前仍有三个职责明确但命名较扁平的 package：

- `component/jsoncodec` 同时提供标准库 JSON 与 Sonic 的 codec 实现。
- `component/tasklog` 把 asynq 的日志接口适配到 FiberHouse 日志系统。
- `component/writer` 提供由 bootstrap 装配的日志文件轮转与异步 writer。

这三个 package 分别属于 codec、task 和 logging 领域。仓库仍在积极演进，本次允许直接改变公开 import path，不保留旧路径兼容层；但目录整理不应顺带改变导出 API、运行行为或既有测试契约。

上一份 component 迁移规格把这三个目录的进一步重组列为当时的非目标。本设计是后续独立决策，不回写或篡改历史规格。

## 目标

1. 将 `codec/`、`task/`、`logging/` 确立为长期可扩展的 component 领域命名空间。
2. 把三个现有 package 迁入最接近其所有权与职责的叶子目录。
3. 明确父命名空间、叶子 package、package 名和依赖方向的规则。
4. 保持现有导出类型、函数、方法、字段、配置和运行语义不变。
5. 用路径归零、全仓编译、已知失败对比和 rename-aware diff 证明迁移完整且未混入行为修改。

## 非目标

- 不重命名 `SonicJsonFastest`、`StdJsonDefault`、`TaskLoggerAdapter`、`NewAsyncDiodeWriter` 等现有导出符号。
- 不改变 JSON fallback、Gin/Fiber codec 装配、task 日志字段、writer 丢弃策略、flush、关闭或错误语义。
- 不实现当前只有 package 声明的 `gojson.go`，也不删除该占位能力。
- 不修复已记录的五个 writer 测试问题或 bootstrap 测试问题。
- 不移动 `component/jsonconvert`；它负责 recovery 数据分类和池化包装，不是 JSON codec backend。
- 不重组 root package 中的 task 核心 API、bootstrap logger 或其他 logging 能力。
- 不提前创建 `codec/yaml`、`logging/hook`、`logging/formatter` 等空目录。
- 不创建父级 facade、registry、自动注册、re-export 或旧路径兼容 shim。
- 不修改 `go.mod`、`go.sum`、YAML 配置键、GlobalManager key 或日志 Origin。

## Component 的分层规则

`component/` 继续是纯命名空间，根目录不提供 Go API。一级目录现在允许两种形态：

1. **可直接导入的能力 package**，例如 `component/cache`、`component/container`。
2. **纯领域命名空间**，例如 `component/database`、`component/codec`、`component/task`、`component/logging`。

分层规则如下：

- 纯领域命名空间不包含 `.go` 文件，不形成 Go package。
- 可导入、可测试和可装配的 API 位于叶子 package。
- 不为目录层级便利而创建 facade 或聚合导入 package。
- 叶子 package 可以依赖 FiberHouse 核心接口，但依赖 root package 的叶子不能被 root 反向导入。
- 只服务于某个叶子 package 的实现细节继续放入该叶子的 `internal/` 子树。
- 目录嵌套只表达领域所有权，不隐式建立 Go 依赖或生命周期所有权。

`codec`、`task`、`logging` 的未来同级扩展是命名空间设计能力，不是本次实施内容。

## 目标结构与精确映射

```text
component/
├── codec/                         # 纯领域命名空间
│   └── json/                      # package jsoncodec
│       ├── gojson.go
│       ├── sonicjson.go
│       └── stdjson.go
├── task/                          # 纯领域命名空间
│   └── logadaptor/                # package logadaptor
│       └── logger_adapter.go
└── logging/                       # 纯领域命名空间
    └── writer/                    # package writer
        ├── async_channel_writer.go
        ├── async_diode_writer.go
        ├── async_diode_writer_test.go
        └── sync_lumberjack_writer.go
```

精确迁移映射：

```text
component/jsoncodec/
  -> component/codec/json/

component/tasklog/
  -> component/task/logadaptor/

component/writer/
  -> component/logging/writer/
```

共移动八个跟踪文件：三个 JSON codec 文件、一个 task logger adaptor 文件、三个 writer 实现文件和一个 writer 测试文件。

## Package 命名决策

### JSON codec

`component/codec/json` 保持 `package jsoncodec`。路径末段与 package 名有意不同，原因是当前实现本身需要同时处理标准库 `encoding/json`、Gin `codec/json` 和 Sonic，未来 Go JSON 实现也会引入另一个 JSON package。使用 `jsoncodec` selector 可以降低同名 import 和审阅歧义。

Go 的默认 import identifier 由 package 声明决定，因此调用方可以继续写：

```go
jsoncodec.SonicJsonFastest()
```

这种受控差异在仓库中已有 `response/pb` 使用 `responsepb` package 名的先例。

### Task logger adaptor

`component/task/logadaptor` 使用 `package logadaptor`。`adaptor` 与仓库现有根 `adaptor/context`、`adaptor/errorhandler` 的目录词汇保持一致。

现有导出符号继续采用标准英语拼写：

```go
logadaptor.NewTaskLoggerAdapter(ctx)
```

本次不把整个仓库从 `adaptor` 改为 `adapter`，也不把 `TaskLoggerAdapter` 改成新的简称。

该 package 归属 task 而不是 logging，因为它实现的是 asynq/task 集成边界，不是通用日志基础设施。它不拥有 task worker、server 或全局 logger 的生命周期。

### Logging writer

`component/logging/writer` 使用单数 `package writer`。一个 package 包含多个 writer 实现不要求使用复数名称；保留单数还能让现有 selector 不变：

```go
writer.NewAsyncDiodeWriter(cfg, filename)
```

该 package 的实现均读取应用日志配置并输出到 lumberjack，属于 logging 领域，而不是通用 `io.Writer` 工具集合。

## API 与兼容性

迁移是直接破坏式 import path 变更：

```text
github.com/lamxy/fiberhouse/component/jsoncodec
  -> github.com/lamxy/fiberhouse/component/codec/json

github.com/lamxy/fiberhouse/component/tasklog
  -> github.com/lamxy/fiberhouse/component/task/logadaptor

github.com/lamxy/fiberhouse/component/writer
  -> github.com/lamxy/fiberhouse/component/logging/writer
```

仓库内调用变化：

- JSON codec 的五个直接生产调用文件只修改 import path，`jsoncodec` selector 不变。
- task logger adaptor 的一个直接生产调用文件同时修改 import path，并把 selector 从 `tasklog` 改为 `logadaptor`。
- writer 的一个直接生产调用文件只修改 import path，`writer` selector 不变。

不保留旧目录、类型别名、转发函数或兼容 package。即使导出符号保持不变，新的 import path 仍会改变 Go package identity 和其中具体类型的身份，仓库外直接消费者必须同步迁移。

## 依赖设计

目标依赖关系：

```text
fiberhouse root --------------------> component/codec/json
bootstrap --------------------------> component/logging/writer
component/logging/writer ----------> appconfig
application/example assembly ------> component/task/logadaptor
component/task/logadaptor ---------> fiberhouse root
```

约束：

- root 可以继续导入 `component/codec/json`，因为 jsoncodec 不依赖 root。
- bootstrap 可以导入 `component/logging/writer`，writer 只依赖 `appconfig` 和第三方输出实现。
- root 的 `task.go` 不得导入 `component/task/logadaptor`；否则会与 logadaptor 对 root 的依赖形成 import cycle。
- task logger adaptor 继续由应用或示例装配层安装到 `asynq.Config.Logger`。
- 三个父命名空间没有 Go package，因此不会增加中间依赖节点。

## 行为与生命周期保持

### JSON codec

- `StdJSON` 与 `SonicJSON` 的字段、方法集和构造器保持不变。
- Sonic 解码失败后回退标准库 JSON 的行为保持不变。
- Fiber encoder/decoder、Gin 全局 JSON API、task payload 和间接 cache/recovery 调用保持现有装配方式。
- `gojson.go` 原样迁移，继续明确为未实现占位。

### Task logger adaptor

- 继续只读取首个 string 参数。
- 继续使用 task Origin 和 `Component=Asynq` 字段。
- `Fatal` 继续继承底层全局日志器的 fatal 语义。
- `Ctx` 公开字段、构造器和五个日志方法保持不变。

### Logging writer

- 同步、channel 和 diode 三种实现的类型与构造器保持不变。
- `application.appLog` 配置读取、`chan|diode` 选择、GlobalManager key 和 lumberjack 参数保持不变。
- goroutine、buffer、丢弃计数、flush、Close 和错误传播行为保持不变。
- 测试文件只随 package 移动，不调整固定等待、路径假设或关闭后写入期望。

## 影响范围

### 生产代码

直接 import 调用共七个文件：

```text
json_fiber_provider.go
json_gin_provider.go
task.go
example_application/application_impl.go
example_application/command/application/application.go
example_application/module/task_impl.go
bootstrap/bootstrap.go
```

间接运行链包括 JSON provider/manager、Fiber/Gin core、task payload、recovery、cache 的 JsonWrapper 消费、task worker 装配、logger bootstrap 和 shutdown；这些调用不应发生行为变化。

### 当前文档

当前有效文档中的旧路径和链接需要更新，范围包括 README、组件参考、feature status、examples，以及 web runtime、background tasks、logging 和 known test failures 指南/记录。

历史设计、历史实施计划和时间点审计记录保留原始路径。新的设计和当前架构文档说明本轮已经解除上一轮的非目标约束。

`.codegraph-qa-out` 只更新描述当前结构的聚焦记录或新增本次分析；不机械改写历史报告。`.codegraph/codegraph.db*`、daemon pid 和日志属于生成数据，不手工修改。

### 配置与依赖文件

`go.mod`、`go.sum`、YAML 配置、常量值和生成源码没有预期内容变化。如果出现差异，必须逐项证明必要性，否则移出迁移范围。

## 已知测试基线

当前完整测试存在六个已记录问题：

- `bootstrap.Test_Config_EnvOverrideAndSingleton`
- `component/writer.TestAsyncDiodeWriter_Write`
- `component/writer.TestAsyncDiodeWriter_MultipleWrites`
- `component/writer.TestAsyncDiodeWriter_ConcurrentWrites`
- `component/writer.TestAsyncDiodeWriter_Close`
- `component/writer.TestAsyncDiodeWriter_WriteAfterClose`

迁移后五个 writer 测试的 package 路径变为 `component/logging/writer`，测试名称、源码和问题归因不变。`ConcurrentWrites` 仍可能因调度边界在不同运行中通过或失败。

本次选择不修复这些测试。完成标准不是要求完整测试全绿，而是：任何完整测试失败都必须属于上述已记录问题，不得新增 package not found、undefined selector、import cycle、构建失败或新的测试失败。

测试中的 `D:/invalid/path/test.log` 在 POSIX 上会成为相对路径并可能生成未跟踪伪影。迁移只移动八个跟踪文件，不携带该伪影；验证后清理新旧测试目录中的生成物并再次检查工作树。

## 实施约束

- 使用 `git mv` 移动跟踪文件并保留历史。
- 不直接移动包含未跟踪测试伪影的整个目录；精确移动文件或先确认目录内容。
- 只修改 `logger_adapter.go` 的 package clause；jsoncodec 和 writer 的 package clause 保持不变。
- 只更新必要的 import path、task selector、当前文档链接和当前结构说明。
- 不对移动文件做无关格式化或换行规范化。
- 不修改生产逻辑、测试逻辑、依赖版本、配置或生成数据。
- 迁移提交按 JSON codec、task logger adaptor、logging writer 和当前文档四个逻辑边界拆分。
- 当前工作树中的任何无关修改不得进入迁移提交。

## 验证设计

### 路径与 package

旧目录和以下旧 import path 必须在产品代码、测试和当前文档中归零：

```text
github.com/lamxy/fiberhouse/component/jsoncodec
github.com/lamxy/fiberhouse/component/tasklog
github.com/lamxy/fiberhouse/component/writer
```

历史规格和时间点审计记录不参与旧路径归零要求。

`component/codec`、`component/task`、`component/logging` 根部不得有 `.go` 文件。`go list` 必须确认：

```text
github.com/lamxy/fiberhouse/component/codec/json       package jsoncodec
github.com/lamxy/fiberhouse/component/task/logadaptor  package logadaptor
github.com/lamxy/fiberhouse/component/logging/writer   package writer
```

### 编译与测试

执行新路径 package 的编译检查以及全仓编译：

```bash
go test ./component/codec/json -run '^$'
go test ./component/task/logadaptor -run '^$'
go test ./component/logging/writer -run '^$'
go test ./... -run '^$'
go build ./...
```

随后执行：

```bash
go test ./...
```

完整测试允许复现已知六项中的任意子集，但不得出现新失败。writer focused tests 不作为全绿门禁，因为本次明确不修复其既有测试债务。

### 差异与范围

```bash
git diff --check
git diff --find-renames --summary
git status --short
```

验证结果应满足：

- 八个目标文件主要识别为 rename。
- 生产差异只包含 import path、task package/selector 和必要文档变化。
- `go.mod`、`go.sum`、配置和运行逻辑没有差异。
- 测试生成伪影已经清理。
- 最终工作树不包含用户已有或本次范围外的修改。

## 计划提交边界

书面设计单独提交。后续实施计划以四个逻辑提交为目标：

1. `refactor: move json codecs into codec namespace`
2. `refactor: move task logger adaptor into task namespace`
3. `refactor: move log writers into logging namespace`
4. `docs: define hierarchical component namespaces`

提交边界可以让每个 import path 迁移独立审阅，同时避免把当前已知测试债务误认为目录迁移引入的行为变化。

## 完成标准

1. 三个目标 package 位于确认后的叶子路径，父领域目录保持纯命名空间。
2. 当前代码、测试和当前文档不再使用旧路径；不保留兼容 shim。
3. 所有现有导出 API、配置、生命周期、错误和并发语义保持不变。
4. 新 package 和全仓编译通过，没有 import cycle 或缺失 selector。
5. 完整测试不出现已记录六项之外的新失败。
6. rename-aware diff 和范围检查证明没有混入测试修复、行为调整、依赖更新或生成数据修改。
7. 当前 component 架构文档正式记录领域命名空间与叶子 package 规则。
