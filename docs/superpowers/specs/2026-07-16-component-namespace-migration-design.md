# Component 命名空间与 Cache/Database 目录迁移设计

**日期：** 2026-07-16

**状态：** 已完成交互设计确认，待书面规格审阅

## 背景

FiberHouse 当前的 `component/` 同时包含根 package `component`、多个独立子 package，以及若干仅表达未来意图的占位目录：

- 根 `component` package 提供基于 Dig 的依赖注入容器。
- `bufferpool`、`jsonconvert`、`mongodecimal` 等是工具或适配实现。
- `jsoncodec`、`validate`、`writer`、`tasklog` 已进入框架或示例装配链。
- `i18n`、`mq`、`rpc` 目前仅为占位能力。
- `cache/` 和 `database/` 位于仓库根目录，但同样属于框架提供、由应用选择装配的可复用基础设施能力。

这种结构使 `component/` 既像 Go package，又像能力分类目录，同时把 cache/database 留在根目录，目录语义不一致。仓库仍处于积极演进阶段，本次允许公开 import path 的破坏性变更，不保留旧路径兼容层。

## 目标

1. 明确定义 `component/` 的长期职责和依赖边界。
2. 将 cache、database 统一归入 component 命名空间。
3. 消除根 `component` package 的特殊形态，将 Dig 容器变为独立组件。
4. 将仅供 MongoDB 使用的 decimal BSON codec 收归 dbmongo 私有实现。
5. 保持缓存、数据库、容器的运行行为、配置、生命周期和错误语义不变。
6. 通过编译、测试、路径归零和差异检查证明迁移完整且没有混入无关修改。

## 非目标

- 不重新设计缓存策略、数据库连接、健康检查或资源关闭机制。
- 不增加 cache/database 的自动注册、自动初始化或 `init()`。
- 不重新命名 `cachelocal`、`cacheremote`、`cache2`、`dbmysql`、`dbmongo` package。
- 不将 `jsoncodec`、`jsonconvert`、`writer`、`tasklog` 等进一步重组为新的多层目录。
- 不修复与目录迁移无关的测试失败、并发问题、生命周期问题或文档债务。
- 不修改 YAML 配置键、GlobalManager 实例 key 或日志来源名称。
- 不保留旧 `cache/`、`database/`、根 `component` 或 `component/mongodecimal` 的兼容 shim。

## Component 的正式定义

`component/` 是 FiberHouse 提供的内置、可选装配、可复用能力的命名空间，不是要求所有子 package 都位于 root 下方的严格依赖底层。

一个目录适合进入 `component/`，应同时满足以下条件：

- 提供框架级可复用能力、适配器或基础设施实现，而不是具体应用业务。
- 可以通过清晰的构造器、接口或调用入口独立理解和使用。
- 由框架核心或应用装配层显式接入，不因目录存在而自动启用。
- 对资源、goroutine、对象池或可变注册表有明确的所有权与生命周期说明。
- 依赖方向可以被明确表达并且不会形成 Go import cycle。

长期边界规则：

1. `component/` 根目录不提供 Go API，不包含 `.go` 文件，仅作为命名空间。
2. 每个一级子目录代表一个可独立理解和装配的能力。
3. 子组件可以依赖 `fiberhouse` root 的核心接口，但依赖 root 的组件不得被 root 直接导入。
4. 依赖 root 的组件由应用注册器或 Provider 装配，不能通过父 `component` package 聚合导入。
5. 只服务于单个组件的辅助实现放在该组件的 `internal/` 子树。
6. 应用 model、service、repository、业务配置和业务 DTO 不进入 `component/`。
7. 占位目录必须明确标记为未实现；目录存在不代表能力可运行。

## 目标目录结构

```text
component/
├── container/                     package container
├── cache/                         package cache
│   ├── cache2/                    package cache2
│   ├── cachelocal/                package cachelocal
│   └── cacheremote/               package cacheremote
├── database/
│   ├── dbmysql/                   package dbmysql
│   └── dbmongo/                   package dbmongo
│       └── internal/
│           └── mongodecimal/      package mongodecimal
├── bufferpool/
├── jsoncodec/
├── jsonconvert/
├── tasklog/
├── validate/
├── writer/
├── i18n/
├── mq/
└── rpc/
```

精确迁移映射：

```text
component/dig_container.go
  -> component/container/dig_container.go

cache/
  -> component/cache/

database/
  -> component/database/

component/mongodecimal/
  -> component/database/dbmongo/internal/mongodecimal/
```

`component/container` 使用职责命名而不是实现名称 `dig`，避免未来替换 DI 实现时再次改变 import path。`DigContainer`、`NewDigContainerOnce`、`NewWrap`、`Invoke` 等导出名称保持不变，只将 selector 从 `component` 改为 `container`。

`mongodecimal` 保持独立 package，以维持 codec 的独立可测试性；它位于 `dbmongo/internal`，因此只有 dbmongo 子树可以使用，不被误认为 database 范围共享能力或公共 API。

## 依赖设计

目标依赖关系：

```text
fiberhouse root
├── component/container
├── component/jsoncodec
├── component/validate
└── component/writer

application assembly
├── component/cache/...
└── component/database/...

component/cache/... ----------------> fiberhouse root
component/database/dbmysql ---------> fiberhouse root
component/database/dbmongo ---------> fiberhouse root
component/database/dbmongo ---------> dbmongo/internal/mongodecimal
```

目录嵌套不会隐式产生 Go package 依赖。`component` 根目录不成为 package，也不导入任何子组件。root 可以安全导入不反向依赖 root 的 `component/container`；cache/database 继续依赖 `fiberhouse.IContext`、配置、日志、常量和 GlobalManager 接口，但只由应用装配层导入。

禁止出现以下依赖：

```text
fiberhouse root -> component/cache -> fiberhouse root
fiberhouse root -> component/database/dbmongo -> fiberhouse root
component facade -> component/cache -> fiberhouse root
```

本次迁移不创建 component facade、公共 registry 或 re-export package。

## API 与兼容性

迁移是直接破坏式变更：

```text
github.com/lamxy/fiberhouse/cache
  -> github.com/lamxy/fiberhouse/component/cache

github.com/lamxy/fiberhouse/cache/cachelocal
  -> github.com/lamxy/fiberhouse/component/cache/cachelocal

github.com/lamxy/fiberhouse/cache/cacheremote
  -> github.com/lamxy/fiberhouse/component/cache/cacheremote

github.com/lamxy/fiberhouse/cache/cache2
  -> github.com/lamxy/fiberhouse/component/cache/cache2

github.com/lamxy/fiberhouse/database/dbmysql
  -> github.com/lamxy/fiberhouse/component/database/dbmysql

github.com/lamxy/fiberhouse/database/dbmongo
  -> github.com/lamxy/fiberhouse/component/database/dbmongo

github.com/lamxy/fiberhouse/component
  -> github.com/lamxy/fiberhouse/component/container
```

除 package identity 与 import selector 外，cache/database 的接口、类型、常量、构造器、方法签名和行为保持不变。`component.DigContainer` 变成 `container.DigContainer`，但类型和方法名称保持不变。

`MongoDecimal` 不再是公共 API。它继续实现 `bson.ValueEncoder` 和 `bson.ValueDecoder`，由 `dbmongo.NewClient` 注册进 BSON registry。

## 装配与数据流

缓存装配保持为：

```text
Application.ConfigGlobalInitializers
  -> cachelocal.NewLocalCache
  -> cacheremote.NewRedisDb
  -> cache2.NewLevel2Cache
  -> GlobalManager 按实例 key 保存
  -> cache.GetCached 按 key 定位 Cache
```

数据库装配保持为：

```text
Application.ConfigGlobalInitializers
  -> dbmysql.NewMysqlDb / dbmongo.NewMongoDb
  -> GlobalManager 保存 client
  -> MysqlModel / MongoModel 按实例 key 定位 client
```

Mongo decimal codec 保持为：

```text
dbmongo.NewClient
  -> dbmongo/internal/mongodecimal.MongoDecimal
  -> 注册到 BSON registry
```

## 生命周期与错误语义

- cache、Redis、MySQL、MongoDB client 继续由应用初始化器创建和持有。
- `Close`、`Wait`、`Rebuild` 和健康检查语义保持不变。
- container 的进程级 singleton、`ResetDigContainer` 与现有并发限制保持不变。
- 配置缺失、连接失败、序列化失败、缓存 miss 等错误继续沿现有路径返回或 panic。
- 不修改现有错误类型、错误消息、日志来源或 fatal 行为。
- 不因目录迁移添加资源创建、连接、重试、关闭或清理逻辑。

## 实施约束

- 使用 `git mv` 保留文件历史。
- 只更新受影响的 import path、package clause、selector、注释示例和当前文档链接。
- 仅对实际修改的 Go 文件执行 `gofmt`，避免扩大换行差异。
- 不机械替换配置中的 `cache`、`database`，也不重命名 root 接口中的 `GetCacheKey`、`GetDBMongoKey` 等领域术语。
- 不手工修改 `.codegraph/codegraph.db*`、daemon pid 或日志；文件移动后由 CodeGraph watcher 或后续索引刷新处理。
- 更新 `.codegraph-qa-out` 中描述当前目录结构的聚焦分析或摘要，不机械重写历史分析记录。
- 当前工作区已有的 response/Protobuf 等无关改动不得进入迁移提交。

## 验证设计

### 路径与目录

产品 Go 代码中的下列旧 import 必须归零：

```text
github.com/lamxy/fiberhouse/cache
github.com/lamxy/fiberhouse/database
github.com/lamxy/fiberhouse/component/mongodecimal
```

根 `github.com/lamxy/fiberhouse/component` 的 Dig 容器 import 也必须归零。`component/` 根目录不得保留 `.go` 文件或旧路径兼容 package。

`go list ./component/...` 应发现 container、cache、database 及其子 package，并且不报告 import cycle 或 package not found。

### 定向测试

```bash
go test ./component/container/...
go test ./component/cache/...
go test ./component/database/...
```

定向验证应确认：

- 原 cache 单元测试随目录迁移后继续通过。
- cache2、cachelocal、cacheremote 的内部 import 正确。
- dbmysql、dbmongo 在不建立真实连接的情况下完成编译。
- dbmongo 可以导入其 `internal/mongodecimal`。
- container 的泛型与 singleton API 编译成功。

### 全仓验证

```bash
go build ./...
go test ./...
git diff --check
```

若迁移前全仓测试已有失败，以失败集合不增加为完成标准。任何新增的 package not found、undefined selector、import cycle、构建失败或测试失败都必须处理。

### 范围验证

- Git 应将 cache 的 12 个文件、database 的 6 个文件以及 Dig container 的 1 个文件主要识别为 rename。
- `mongodecimal` 移入 dbmongo 的 internal 子树。
- 更新必要的 application、command、model、service、task、context、文档和 CodeGraph 当前结构摘要。
- `go.mod`、`go.sum`、YAML 配置和运行逻辑原则上不产生语义变化。
- 审阅暂存差异，确保不包含当前工作区已有的无关改动。

## 完成标准

1. 目标目录结构与本设计一致，旧 package 路径不再存在。
2. component 的正式定义和依赖规则写入当前架构文档。
3. 所有仓库内产品代码、测试、示例和当前文档使用新路径。
4. 定向构建与测试通过；全仓验证不新增失败。
5. 没有 import cycle、旧路径残留、兼容 shim 或无关行为修改。
6. 迁移提交只包含本设计范围内的文件。
