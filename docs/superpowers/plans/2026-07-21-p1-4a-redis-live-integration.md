# P1-4a: Redis cache live integration

## 背景与问题

`.codegraph-qa-out/readme-current-status-first-two-rows-stability-research-2026-07-18.md`
「九、推荐执行顺序」第 9 项候选（P1-4 的第一个小批次）。原文（第 213-230
行）：

> ### P1-4：复用 smoke 服务的 live integration
>
> 推荐给 live tests 增加显式 `liveintegration` build tag，并只在 smoke job
> 中定向运行。普通 quality/race 不连接外部服务。
>
> 推荐拆成小批次：
>
> 1. Redis cache：Ping、SET、GET、DEL、Close 后失败；使用唯一 key。
> ...
>
> CI 不需要新增容器。可以在现有 smoke job 增加类似的定向步骤，并给整个
> live test 命令设置约 90 秒上限。所有测试必须使用唯一资源名、短 context
> 和 `t.Cleanup`，避免并发 PR 或失败重跑相互污染。
>
> 不推荐：
>
> - 只运行 `redis-cli`、`mysql` 或 `mongosh` 探针，因为它只能证明容器正常，
>   不能证明仓库 client 配置和读写路径正常；
> - 为测试增加业务示例 API，因为这会把组件验证耦合到示例路由；
> - 第一批同时覆盖所有重建、故障注入和高并发场景，因为这会把快速验证扩成
>   长期集成项目。

**本次范围**：只做 P1-4 的第一个小批次——Redis cache live integration。
asynq（DB 15）、MySQL、MongoDB 三个小批次留待后续各自独立立项，不在本次
范围内。

**现状证据**：
- `.github/workflows/go1.yml` 的 `smoke` job（第 59-136 行）已经用 GitHub
  Actions `services` 启动 `redis:8.2.7-bookworm`（端口 6379）、
  `mongodb:8.0.26-noble`（端口 27037）、`mysql:8.4.10-oraclelinux9`（端口
  3306），但目前只做了 HTTP smoke（`curl /example/hello/world`），从未真正
  调用仓库的 Redis/MySQL/MongoDB client 执行读写。
- `example_config/application_test.yml` 的 `cache.redis` 段（第 108-110
  行）配置为 `host: 127.0.0.1`、`port: 6379`、`db: 0`，与 CI service 容器
  端口完全对应；本机也已用
  `docs/docker_compose_db_redis_yaml/docker-compose.yml`（略有出入：
  redis 容器名不同，MySQL 密码 `root:root`，均已核实与配置一致）起了对应
  的本地容器，可在本机独立验证。
- `component/cache/cacheremote/redis_cache_test.go` 已有
  `newTestRedisDb(t)` 辅助函数，构造一个指向 `127.0.0.1:6379`、`db: 0` 的
  `RedisDb`；但该文件现有两个测试
  （`TestRedisDb_CloseAndPostCloseErrors`、
  `TestRedisDb_ConcurrentCloseIsIdempotentAndPanicFree`）利用
  `redis.NewClient()` 的懒连接特性，从未真正发起网络 I/O，只验证 `Close()`
  的 CAS 幂等语义——不是本次要新增的 live 读写验证，且这两个测试没有
  build tag，普通 `go test ./...`（quality/race job）也会执行，如果本次
  为它们所在的整个套件添加 `liveintegration` tag，反而会让这两个不需要
  外部依赖的测试也被排除出 quality/race job，属于范围外的行为改变。

**结论**：不能给整个 `redis_cache_test.go` 文件加 build tag，而应该把新增
的 live 测试放进*新文件*，只有新文件带 `liveintegration` tag。

## 修复方向（这是纯测试新增，不修改生产代码）

1. 在 `component/cache/cacheremote` 包内新增一个带 `//go:build
   liveintegration` 标签的测试文件，验证 `RedisDb` 的 `Get`/`Set`/
   `Delete`/`Close` 在连接真实 Redis 容器时的行为：Ping（通过
   `NewRedisDb` 构造成功即隐含连接可达，或显式调用 `Client.Ping`）、
   `Set` 写入一个带唯一后缀的 key、`Get` 读回验证值相同、`Delete` 删除、
   删除后 `Get` 返回 `cache.ErrKeyNotFound`（不是 Redis 层面的
   `nil`，需要核实 `RedisDb.Get` 对未找到 key 的实际错误类型）、`Close`
   后再次调用任意方法验证返回 `cache.ErrCacheClosed`（复用现有语义，不是
   本次新增行为，只是用真实连接场景再验证一次）。
2. 使用独立的 Redis DB 编号（建议 `db: 14`，避开研究文件为下一批次
   asynq 预留的 `db: 15`，也避开现有单元测试使用的 `db: 0`），避免与其他
   批次或现有测试的键空间冲突。
3. Key 名称必须唯一（使用 `t.Name()` + 随机后缀或时间戳拼接），并用
   `t.Cleanup` 确保测试产生的 key 无论成功失败都会被删除，避免污染并发
   CI 运行或重跑。
4. 整个测试函数应使用短 `context.Context`（建议单次操作 5 秒超时，参照
   仓库其他地方的 context 超时惯例，例如 `redis_cache.go` 的
   `IsHealthy` 用 15 秒、其他地方常见 5-10 秒）。

## Global Constraints

- Go 版本与既有 `go.mod` 一致，不升级/新增依赖。
- 不修改 `component/cache/cacheremote/redis_cache.go` 或任何其他生产
  代码；不修改 `component/cache/cacheremote/redis_cache_test.go` 现有
  内容（新测试放在新文件）。
- 新文件必须带 `//go:build liveintegration` build tag（Go 1.17+ 语法），
  确保 `go test ./...`（无 tag）不会执行这些测试，只有显式传入
  `-tags=liveintegration` 才会编译进测试二进制。
- 新增测试不得依赖示例应用（`example_application`/`example_main`）的任何
  业务路由或 API，直接用 `component/cache/cacheremote` 包内部的
  `NewRedisDb`/`RedisDb` 验证。
- 使用唯一资源名（key 前缀 + 随机/时间戳后缀）、短 context、`t.Cleanup`，
  避免并发 PR 或失败重跑相互污染（研究文件明确要求）。
- 不新增业务示例 API，不做故障注入/高并发/重建场景（研究文件"不推荐"
  部分明确排除）。
- CI 修改限定在 `.github/workflows/go1.yml` 的 `smoke` job 内增加一个
  定向执行 `-tags=liveintegration` 的步骤，不新增额外的 GitHub Actions
  服务容器（现有 `smoke` job 已经有 Redis 容器）、不修改 `quality`/`race`
  job。

## File Structure

```
component/cache/cacheremote/redis_cache_live_test.go   # 新增：liveintegration tag 测试
.github/workflows/go1.yml                               # 修改：smoke job 新增定向执行步骤
```

## 任务拆分

### 任务 1：新增 Redis live integration 测试

**目标文件**：新增 `component/cache/cacheremote/redis_cache_live_test.go`
（package `cacheremote`，与现有 `redis_cache_test.go` 同包）。

**实现思路**：

```go
//go:build liveintegration

package cacheremote

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	"github.com/lamxy/fiberhouse/component/cache"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func newLiveTestRedisDb(t *testing.T) *RedisDb {
	t.Helper()
	cfg := appconfig.NewAppConfig().LoadDefault(map[string]interface{}{
		"live.redis.host":     "127.0.0.1",
		"live.redis.port":     "6379",
		"live.redis.db":       14, // 独立于单元测试(db 0)与预留给 asynq 批次(db 15)
		"live.redis.poolSize": 5,
	})
	logger := zerolog.Nop()
	appCtx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	origin, err := NewRedisDb(appCtx, "live.redis")
	require.NoError(t, err)
	rd := origin.(*RedisDb)
	t.Cleanup(func() { _ = rd.Close() })
	return rd
}

func TestLive_RedisDb_PingSetGetDeleteClose(t *testing.T) {
	rd := newLiveTestRedisDb(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.True(t, rd.PingTry(ctx), "must be able to reach the live Redis container")

	key := fmt.Sprintf("p1-4a-live-%s-%d", t.Name(), time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = rd.Delete(cleanupCtx, key)
	})

	co := cache.NewCacheOption(...) // 参照仓库其他 cache 测试如何构造 CacheOption
	value := "p1-4a-live-value"

	require.NoError(t, rd.Set(ctx, key, value, co))

	got, err := rd.Get(ctx, key, co)
	require.NoError(t, err)
	require.Equal(t, value, got)

	require.NoError(t, rd.Delete(ctx, key))

	_, err = rd.Get(ctx, key, co)
	require.ErrorContains(t, err, ...) // 核实 RedisDb.Get 对未命中 key 的真实错误类型/消息

	require.NoError(t, rd.Close())
	closeErr := rd.Close()
	require.ErrorIs(t, closeErr, cache.ErrCacheClosed)
}
```

实现者需要：
- 先读取 `component/cache/cache_option.go`（或等价文件）确认
  `cache.CacheOption`/`cache.NewCacheOption` 的真实构造方式（上面是示意，
  不保证签名完全正确）；可参照
  `component/cache/cachelocal/local_cache_test.go`、既有
  `component/cache/cacheremote/redis_cache_test.go` 中如果有构造
  `CacheOption` 的既有用法。
- 核实 `RedisDb.Get` 在 key 不存在时返回的具体错误值/类型（读取
  `redis_cache.go` 的 `Get`/`getInternal` 实现，确认是
  `cache.ErrKeyNotFound` 还是 `cache.ErrRedisNil` 或其他），断言必须针对
  实际返回值，不能凭空假设。
- 若发现 `CacheOption` 构造需要更多必需字段（例如 JSON codec、TTL）才能
  让 `Set`/`Get` 正常工作，比照 `cachelocal`/既有 `cacheremote` 测试文件
  的做法补全。

**验收标准**：
- `go build ./...`（不带 tag）通过，新文件因 build tag 被排除，不影响
  普通构建。
- `go vet ./...`（不带 tag）不扫描到新文件的语法问题（若 vet 默认不看
  build-tag 排除的文件，只需确认默认构建不受影响）；额外执行
  `go vet -tags=liveintegration ./component/cache/cacheremote/...`
  确认新文件本身语法正确。
- 本机容器已启动的前提下运行：

```bash
go test -tags=liveintegration ./component/cache/cacheremote/... -run 'TestLive_RedisDb' -v -count=1
```

  必须通过，且实际发起了真实网络 I/O（不是像现有 `newTestRedisDb` 那样
  依赖懒连接从不触发失败）。
- 不带 tag 的默认测试命令确认新文件被排除、现有测试不受影响：

```bash
go test ./component/cache/cacheremote/... -v -count=1
```

  只应看到 `TestRedisDb_CloseAndPostCloseErrors`、
  `TestRedisDb_ConcurrentCloseIsIdempotentAndPanicFree`，不应出现
  `TestLive_RedisDb_PingSetGetDeleteClose`。
- 全量套件（不带 tag）不因此改动新增失败：

```bash
go build ./... && go vet ./... && go test ./... -count=1
```

- 测试执行时间应在数秒内完成（单次真实 Redis 读写操作，不应引入长
  sleep 或大超时）。
- 用 `t.Cleanup` 确保 key 与连接都被清理，重复运行该测试（`-count=3`）
  应保持稳定通过，不因为上次遗留的 key 或连接状态导致失败。

### 任务 2：CI smoke job 新增定向执行步骤

**目标文件**：`.github/workflows/go1.yml`。

**修改内容**：在 `smoke` job 的步骤列表中，`Check HTTP path` 之后（或
`Build example`/`Start example` 之前均可，实现者可自行选择合理位置，但
不要把它插在 `Start example`/`Check HTTP path`/`Stop example` 三步中间，
避免打断既有 smoke 流程的连续性；建议放在 `Check HTTP path` 成功之后、
`Stop example` 之前，与主 smoke 校验解耦，即使 live integration 测试失败
也不应该影响后续 example 进程的正常停止清理），新增一步：

```yaml
      - name: Run live integration tests
        timeout-minutes: 2
        run: go test -tags=liveintegration ./component/cache/cacheremote/... -run 'TestLive_' -v -count=1
```

关键点：
- 用 `timeout-minutes: 2`（对应研究文件"约 90 秒上限"的建议，GitHub
  Actions 的 step 级超时最小粒度是分钟，2 分钟是比 90 秒稍宽松但同数量级
  的合理取整；若实现者判断 1 分钟已足够稳定通过可以收紧，但不应超过 2
  分钟）。
- `-run 'TestLive_'` 精确匹配本次新增的测试函数命名前缀（若实现者在
  任务 1 中使用了不同的命名前缀，这里的 `-run` 参数需要同步调整为一致
  的前缀）。
- 不需要新增 GitHub Actions `services` 容器，`smoke` job 已有的
  Redis/MongoDB/MySQL 容器可直接复用。
- 只影响 `smoke` job，不得修改 `quality`、`race` 两个 job 的任何步骤。

**验收标准**：
- YAML 语法正确（可用 `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/go1.yml'))"`
  或等价方式本地校验，不要求本地能真正跑 GitHub Actions）。
- 新增步骤只出现一次，位置符合上述要求（不打断 `Start example`/
  `Check HTTP path`/`Stop example` 的连续性）。
- `quality`、`race` 两个 job 的 YAML 内容与本次改动前逐字节一致（只有
  `smoke` job 有变化）。
- diff 范围只涉及 `.github/workflows/go1.yml` 这一个文件，且只新增这一个
  步骤块，不改写其他既有步骤。

## 最终验证（全分支）

在合并回 `main` 之前执行（本机已具备三个容器，可以直接验证 live 部分）：

```bash
go build ./...
go vet ./...
go test ./... -count=1
go test -tags=liveintegration ./component/cache/cacheremote/... -run 'TestLive_' -v -count=3
```

全部通过后进入 `superpowers:requesting-code-review` 做全分支审查，
Critical/Important 问题必须为 0 才能合并。审查时应额外确认 CI YAML 的
改动范围与语法正确性（审查者可能无法真正触发 GitHub Actions 运行，应
以静态检查 + 本地 `-tags=liveintegration` 命令的等价性作为审查依据）。

## 停止条件

- 若本机的 Redis 容器（`redis-cli ping` 应返回 `PONG`）不可达，先停止并
  上报，不要在没有真实连接的情况下伪造这个测试的"通过"（例如不能退化成
  仅依赖懒连接、从不实际读写的验证方式，那样就失去了 live integration 的
  意义）。
- 若发现 `RedisDb.Get`/`Set`/`Delete` 在真实连接下的行为与 `redis_cache.go`
  当前实现的预期不符（例如布隆过滤器/熔断器保护默认开启导致读写路径
  与预想不同），先停止并上报现象，不擅自修改生产代码去"配合测试通过"——
  若确认是测试配置需要关闭保护机制（`protection.enable: false` 之类），
  可以在测试的 `LoadDefault` 配置中调整，这属于测试范围内的调整，不算
  修改生产代码。
- 不得为了让 CI 更快而缩短 GitHub Actions 现有的容器健康检查
  (`--health-cmd` 等)配置，本次范围不touch `services` 定义本身。
