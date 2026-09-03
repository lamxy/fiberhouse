# Go 性能剖析方法论

> **定位**：这份文档回答「**怎么做**」——工具、脚本、指标、场景、步骤。
>
> 相关文档：
> - [压测操作手册](./bench-profiling-runbook.md) — 本项目的具体操作步骤与观察清单
> - [Fiber 压测结果](./fiber-profiling-findings.md) — 一次完整分析的实例与教学附录
> - [压测命令设计](./bench-command-design.md) — bench 命令的设计决策
>
> 本文所有数据均为本机实测（16 核 i5-13500H / Go 1.26.5），非构造范例。

---

## 一、工具准备

### 1.1 工具清单

| 工具 | 来源 | 用途 | 是否必须 |
|---|---|---|:-:|
| `go tool pprof` | Go 自带 | 采集与分析五类 profile | ✅ |
| `go test -bench` | Go 自带 | 微基准（函数级） | ✅ |
| `net/http/pprof` | 标准库 | 暴露线上 profile 端点 | ✅ |
| `GODEBUG=gctrace=1` | Runtime | GC 逐次追踪 | ✅ |
| `benchstat` | `golang.org/x/perf` | 基准结果统计比对 | 推荐 |
| Graphviz | 系统包 | pprof 图形输出 | 可选 |

安装非自带部分：

```bash
go install golang.org/x/perf/cmd/benchstat@latest
sudo apt install graphviz          # 仅 -web / -svg 需要
```

### 1.2 开启 pprof 端点

**关键**：block 与 mutex profile **默认关闭**，不显式开启则采集结果为空。

```go
runtime.SetBlockProfileRate(10000)   // 纳秒；每 10µs 阻塞采样一次
runtime.SetMutexProfileFraction(100) // 分母；每 100 次争用采样一次
```

本项目已封装为配置项（`application.pprof.*`），见 `pprof_config.go`：

```yaml
pprof:
  enable: true
  host: "127.0.0.1"      # 生产环境务必回环；绑 0.0.0.0 等于对外暴露诊断端点
  port: 6060
  blockRate: 10000
  mutexFraction: 100
  writeTimeout: 120      # 必须 > CPU profile 的 seconds 参数，否则采集被截断
```

> **为什么不用 `import _ "net/http/pprof"`**：blank import 会把路由注册到 `DefaultServeMux`，若业务也用它就等于把诊断端点暴露在业务端口上。本项目改用独立 `http.ServeMux` + 独立端口，可单独关闭、单独鉴权。

### 1.3 核心命令与参数

#### 采集

```bash
# CPU：seconds 是「持续采样时长」，期间必须有稳定负载
go tool pprof -proto -output=cpu.pb.gz \
  "http://127.0.0.1:6060/debug/pprof/profile?seconds=45"

# Heap / Goroutine / Block / Mutex：瞬时快照，无 seconds
go tool pprof -proto -output=heap.pb.gz  http://127.0.0.1:6060/debug/pprof/heap
go tool pprof -proto -output=block.pb.gz http://127.0.0.1:6060/debug/pprof/block
go tool pprof -proto -output=mutex.pb.gz http://127.0.0.1:6060/debug/pprof/mutex

# Goroutine 只需看总数与栈时，debug=1 的纯文本更方便
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head -1
```

> ⚠️ **一定要用 `go tool pprof -proto -output=`，不要用 `curl -o`**。
> curl 直接存档常得到损坏文件（`gunzip: not in gzip format`），因为端点返回的编码与 curl 的处理不一定匹配。`-proto` 让 pprof 自己完成解析与落盘。

#### 分析

| 命令 | 作用 | 何时用 |
|---|---|---|
| `-top -nodecount=N` | 按 flat 排序前 N 名 | 找「谁在烧 CPU」 |
| `-top -cum` | 按 cum 排序 | 找「哪条路径贵」 |
| `-peek='<正则>'` | 展开单一函数的上下游 | **拆解 cum，判断成本归属** |
| `-list='<正则>'` | 逐行源码标注耗时 | 定位到具体行 |
| `-lines` | 以「行」为节点粒度 | 函数被内联时 |
| `-base old.pb.gz` | 差分（新 − 旧） | **判断泄漏** |
| `-focus` / `-ignore` | 只看 / 排除匹配路径 | 滤掉背景噪音 |
| `-sample_index=` | 切换样本维度 | heap 的四种视角 |
| `-http=:8000` | 浏览器交互（火焰图） | 探索性分析 |

```bash
go tool pprof -top -nodecount=20 cpu.pb.gz
go tool pprof -peek='fiberzerolog.New.func1$' cpu.pb.gz    # $ 锚定，避免匹配到衍生符号
go tool pprof -list='validateID' cpu.pb.gz
go tool pprof -base heap_t1.pb.gz heap_t2.pb.gz
go tool pprof -http=:8000 cpu.pb.gz
```

#### heap 的四个 `-sample_index`

| 值 | 含义 | 回答的问题 |
|---|---|---|
| `inuse_space` | 当前占用字节（默认） | **谁占着内存不放**（泄漏） |
| `inuse_objects` | 当前占用对象数 | 是否有大量小对象 |
| `alloc_space` | 累计分配字节 | **谁在制造 GC 压力**（速率） |
| `alloc_objects` | 累计分配对象数 | 分配次数热点 |

**这是最常被混淆的一组。** `inuse` 是「存量」，`alloc` 是「流量」。一个函数可以分配 6GB（`alloc_space` 极高）却只占用 1MB（`inuse_space` 极低）——那不是泄漏，是 GC 压力。

---

## 二、脚本如何设计

### 2.1 原生工具链 vs 并发测试工具

| 方案 | 适用 | 不适用 |
|---|---|---|
| `go test -bench` | 函数级微基准、A/B 对比 | 端到端、中间件链、真实 I/O |
| `wrk` / `hey` / `k6` | 标准 HTTP 压测 | 需要多步骤业务流程（建数据→读→改） |
| **自写 Go 压测命令** | **有状态的 CRUD 业务流** | 只想快速打单一 URL |

本项目选择**自写**（`bench` 命令），原因是被测 API 需要三阶段：播种数据 → 按权重混合读写 → 清理。`wrk` 打单一 URL 做不到，而用 Go 写可直接复用项目的数据结构与配置。

> 若只是打单一固定 URL，`wrk -t8 -c256 -d60s http://...` 更省事，不必自写。

### 2.2 开环 vs 闭环：最重要的设计决策

| | 闭环（固定并发） | 开环（固定速率） |
|---|---|---|
| 发送逻辑 | 等前一个响应再发下一个 | 按墙钟时间定速发送，不等响应 |
| 服务器变慢时 | 客户端**自动降速** | 请求**堆积** |
| 延迟测量 | **系统性低估** | 如实反映 |

闭环的问题叫**协调遗漏（coordinated omission）**：系统越慢，施加的压力反而越小，于是测不出真实延迟。

**开环实现要点**（`bench_runner.go`）：

```go
ticker := time.NewTicker(time.Second / time.Duration(rps))
for {
    select {
    case <-ticker.C:
        select {
        case jobCh <- struct{}{}:      // 投递节拍
        default:
            atomic.AddInt64(&dropped, 1)  // worker 跟不上，记录丢弃
        }
    case <-ctx.Done():
        return
    }
}
```

> **`dropped > 0` 表示测量结果不可信**——说明压测客户端自己成了瓶颈，测到的是客户端能力而非服务端性能。必须提高 `--workers` 或降低 `--rps` 重测。

### 2.3 三个必踩的坑

**① `MaxIdleConnsPerHost` 默认是 2**

不调整的话，连接复用会成为客户端瓶颈，测到的是压测工具的限制。

```go
tr := &http.Transport{
    MaxIdleConns:        workers * 2,
    MaxIdleConnsPerHost: workers * 2,   // 默认 2，必改
    IdleConnTimeout:     90 * time.Second,
}
```

**② 必须读完并关闭 response body**

否则连接无法复用，每个请求都新建 TCP 连接。

```go
io.Copy(io.Discard, resp.Body)   // 读完
resp.Body.Close()                // 才能回池
```

**③ 每个 worker 独立 `rand.Rand`**

共享全局 `rand` 会产生锁争用，**污染被测系统的 mutex profile**——你会在 profile 上看到压测工具自己的锁。

```go
rng := rand.New(rand.NewSource(seed + int64(workerID)))
```

### 2.4 其他设计决策

- **warmup 不可省**：`sync.Pool` 填充、本地缓存预热、连接池建立，前几秒数据严重偏离稳态。
- **cleanup 用独立 context**：压测 context 可能已因超时或 Ctrl-C 取消，清理仍应尽力完成。
- **分位数用全量排序**：3 万个 `time.Duration` 仅 240KB，比 HDR 直方图精确且零依赖。
- **删除操作不进主循环**：会让 ID 池失效并污染缓存行为，只在 cleanup 阶段执行。

---

## 三、指标：如何确定，以及为什么

### 3.1 先确定「要回答什么问题」

指标不是越多越好。**先有问题，再选指标**：

| 问题 | 主指标 | 佐证指标 |
|---|---|---|
| 系统慢在哪？ | p50/p90/p99 延迟 | CPU 采样率、block profile |
| 能扛多少量？ | actual RPS、error rate | dropped 计数 |
| 内存会不会爆？ | heap 差分、GC 频率 | `inuse_space` |
| 是不是锁问题？ | mutex/block profile | CPU 中的 `futex` 占比 |
| 是不是 GC 问题？ | gctrace 的 GC CPU% | 存活堆趋势 |

### 3.2 负载侧指标

| 指标 | 解释 | 为什么看它 |
|---|---|---|
| **p50** | 中位数延迟 | 典型用户体验；平均值会被长尾拉偏，**不要用平均值** |
| **p90 / p99** | 尾延迟 | 真实痛点；p99 = 每 100 个请求有 1 个这么慢 |
| **max** | 最坏情况 | 常对应 GC STW 或锁等待尖峰 |
| **actual RPS** | 实际达成速率 | 与 target 差距大 → 系统或客户端已饱和 |
| **error rate** | 错误率 | 非零时延迟数据需重新审视（失败请求通常更快返回） |
| **dropped** | 客户端丢弃的节拍 | **非零则整份数据不可信** |

> **为什么不用平均延迟**：平均值把一个 500ms 的请求和九十九个 1ms 的请求混成 6ms，掩盖了真实问题。分位数才能揭示分布。

### 3.3 五类 Profile 各自回答什么

| Profile | 测量对象 | 单位 | 典型问题 |
|---|---|---|---|
| **CPU** | 谁在**消耗 CPU 时间** | 秒 | 计算热点、低效算法 |
| **Heap** | 谁在**占用/分配内存** | 字节 / 对象 | 泄漏、GC 压力 |
| **Goroutine** | 有多少 goroutine、卡在哪 | 个 | goroutine 泄漏 |
| **Block** | 谁在**等待同步原语** | 时间 | channel、锁、WaitGroup 阻塞 |
| **Mutex** | 谁在**争抢锁** | 时间 | 锁竞争 |

**CPU 只能发现「忙」的问题，发现不了「等」的问题。** 一个被锁阻塞的系统，CPU profile 上可能什么都看不出来——这正是 block/mutex profile 存在的理由。

#### flat 与 cum 的区别（理解一切的基础）

- **flat**：该函数**自身**执行消耗的时间，不含它调用的其他函数
- **cum**（cumulative）：该函数**及其所有下游**的总消耗

```
flat 高 → 这个函数自己很慢（计算密集、低效实现）
cum 高但 flat 低 → 它只是「入口」，真正的成本在下游
```

> ⚠️ **中间件的 cum 必然很高**，因为它包含 `Next()` 之后的整条链。判断中间件自身开销**必须**用 `-peek` 减去 `Next` 分支。详见 §5.3 的实例。

#### block / mutex 的单位很反直觉

它们显示的是**累积等待时间**，可能远超墙钟时间——因为是所有 goroutine 的等待相加。

```
13.03hrs 67.66%  sync.(*Mutex).Lock
```

70 秒的压测出现 13 小时？合理：数百个 goroutine 各等数十毫秒，累加起来就是这个量级。**看相对占比，不要看绝对值。**

### 3.4 GC 指标（gctrace 逐栏解释）

```bash
GODEBUG=gctrace=1 ./server 2>&1 | grep '^gc '
```

```
gc 161 @208.982s 0%: 0.12+1.3+0.033 ms clock, ... 45->46->23 MB, 46 MB goal, 16 P
   │      │      │      │   │   │                  │   │   │      │
   │      │      │      │   │   └ 标记终止 STW     │   │   │      └ 目标堆大小
   │      │      │      │   └ 并发标记耗时          │   │   └ 标记后存活堆 ★
   │      │      │      └ 标记开始 STW              │   └ 标记后堆大小
   │      │      └ GC 占总 CPU 的百分比 ★           └ 标记前堆大小
   │      └ 程序启动后经过的时间
   └ 第几次 GC
```

| 指标 | 健康标准 | 异常含义 |
|---|---|---|
| GC CPU% | < 10% | 过高 → 分配速率太大 |
| STW（两个 clock 值） | < 1ms | 过长 → 堆过大或有大量 finalizer |
| **存活堆（第三个 MB 值）** | **稳定不增长** | **持续上升 = 泄漏** |
| GC 频率 | 秒级 | 毫秒级 → 分配过快 |

> **存活堆是判断泄漏最直接的指标**，比 heap profile 更快看出趋势。

---

## 四、两种场景

### 4.1 场景一：单元测试性能测试（微基准）

**适用**：函数级优化、A/B 对比、回归检测。**绕开 I/O，只测代码本身。**

#### 基本用法

```bash
go test -run='^$' -bench='BenchmarkXxx' -benchmem -benchtime=2s -count=10 ./pkg/
```

| 参数 | 含义 |
|---|---|
| `-run='^$'` | **不匹配任何测试**，只跑 benchmark（否则单元测试也会执行，污染计时） |
| `-bench='正则'` | 选择 benchmark；`.` 表示全部 |
| `-benchmem` | **输出内存分配**（B/op、allocs/op），强烈建议始终加上 |
| `-benchtime=2s` | 每个 benchmark 至少跑 2 秒（或用 `100x` 指定次数） |
| `-count=10` | 重复 10 轮，供 `benchstat` 做统计检验 |
| `-cpuprofile` / `-memprofile` | 同时产出 profile |

#### 实测输出与解读

```
goos: linux
goarch: amd64
pkg: github.com/lamxy/fiberhouse/globalmanager
cpu: 13th Gen Intel(R) Core(TM) i5-13500H
BenchmarkGlobalManager_ReadHeavy-16   21059746   67.30 ns/op   24 B/op   2 allocs/op
BenchmarkGlobalManager_PureGet-16     13539175   89.35 ns/op   15 B/op   1 allocs/op
                              │        │           │            │         │
                              │        │           │            │         └ 每次操作分配次数
                              │        │           │            └ 每次操作分配字节
                              │        │           └ 每次操作耗时
                              │        └ 总共执行了多少次
                              └ GOMAXPROCS（16 核）
```

**怎么读**：
- `ns/op` 是主指标，但**单看它会误判**——要同时看 `allocs/op`
- `allocs/op` 往往是优化的真正入口：减少分配通常比优化计算收益更大
- 上例中 `ReadHeavy` 比 `PureGet` **更快**（67 vs 89 ns），因为前者有缓存命中路径

#### A/B 对比要用 benchstat，不要肉眼比

单次结果有噪声，直接比大小可能得出相反结论。

```bash
go test -run='^$' -bench=. -count=10 ./pkg/ > old.txt
# ... 改代码 ...
go test -run='^$' -bench=. -count=10 ./pkg/ > new.txt
benchstat old.txt new.txt
```

`benchstat` 会给出差异百分比与 p 值，**p > 0.05 表示差异不显著**（可能只是噪声）。

#### 从 benchmark 产出 profile

```bash
cd globalmanager
go test -run='^$' -bench='BenchmarkGlobalManager_PureGet' \
  -cpuprofile=bench_cpu.out -benchtime=2s .
go tool pprof -top -nodecount=10 bench_cpu.out
```

> ⚠️ **profile 文件写在被测 package 目录下**，不是当前目录；同时会留下一个 `<pkg>.test` 二进制文件。分析完记得清理。

**实测输出**：

```
Duration: 2.52s, Total samples = 2760ms (109.58%)

      flat  flat%   sum%        cum   cum%
     660ms 23.91% 23.91%     1110ms 40.22%  internal/sync.(*HashTrieMap[...]).Load
     170ms  6.16% 30.07%      400ms 14.49%  runtime.mallocgcTiny
     150ms  5.43% 35.51%      280ms 10.14%  fmt.(*fmt).fmtInteger
     150ms  5.43% 40.94%      570ms 20.65%  fmt.(*pp).doPrintf      ← ⚠️
     120ms  4.35% 45.29%      170ms  6.16%  sync.(*Pool).Put
```

**⚠️ 这里有一个典型陷阱**：`fmt.doPrintf` 占了 20.65%，但那**不是被测代码**。用 `-peek` 确认来源：

```
$ go tool pprof -peek='doPrintf' bench_cpu.out
                     0.57s   100% |   fmt.Sprintf
     0.15s  5.43%      0.57s 20.65%  | fmt.(*pp).doPrintf
```

`fmt.Sprintf` 是 **benchmark 循环自己**用来构造 key 的，属于测试脚手架开销。

> **教训**：微基准的 profile 里混有测试代码自身的成本。**优化前必须先确认热点属于被测代码**，否则会去优化 `fmt.Sprintf`。正确做法是把 key 预先生成到 slice，在计时循环外完成。

### 4.2 场景二：API 压测（端到端）

**适用**：验证真实链路——中间件、序列化、数据库、缓存全部参与。

#### 与微基准的根本差异

| | 微基准 | API 压测 |
|---|---|---|
| 测什么 | 单一函数 | 完整请求链路 |
| I/O | 无 | 有（DB、缓存、网络） |
| 主要风险 | 测到测试脚手架 | **测到 DB 而非框架** |
| 指标 | ns/op、allocs/op | 分位数延迟、RPS |

#### 最大的陷阱：DB 先饱和，框架成本被稀释

本项目实测（`--rps=6000 --workers=512`）：

```
Duration: 45s, Total samples = 74.92s (166.49%)     ← 16 核仅 10.4% 利用率

17.69s 23.61%  Syscall6
13.81s 18.43%  runtime.futex
16.98s 22.66%  runtime.findRunnable    ← 调度器空转找不到活干
```

`findRunnable` 占 22.66%，是「CPU 在等 I/O」的决定性证据。**这种状态下 profile 上所有框架函数的百分比都被稀释了**，得出的优化结论没有意义。

**判断方法**：先看采样率。

```
Total samples = 74.92s (166.49%)   → 16 核只用 10.4%，远未饱和
Total samples = 26.55s (66.37%)    → 单核满载附近，框架代码已显形
```

**让框架显形的三个手段**：

```bash
# 1. 提高缓存命中路径占比，把 DB 踢出画面
go run . bench --rps=3000 --w-cache-get=80 --w-get=10 --w-list=10 \
               --w-create=0 --w-update=0

# 2. 用 -focus 只看自己的代码
go tool pprof -focus='fiberhouse' -top -nodecount=20 cpu.pb.gz

# 3. 若仍打不上去，改用微基准或 mock 数据层
```

---

## 五、测试步骤参考

### 5.1 完整流程（API 压测）

```bash
# ── 步骤 0：确认依赖 ──────────────────────────────
docker ps --format '{{.Names}}\t{{.Status}}' | grep -iE "mongo|redis"

# ── 步骤 1：启动服务（终端 1）─────────────────────
# 必须从仓库根目录启动（配置路径相对 CWD）
# 必须显式指定环境，否则回落到 dev
cd /path/to/repo
GODEBUG=gctrace=1 APP_ENV_application_env=test go run ./example_main 2>gctrace.log

# ── 步骤 2：确认就绪并记录基线 ────────────────────
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:6060/debug/pprof/
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head -1
# → goroutine profile: total 38          ★ 记下这个数

# ── 步骤 3：启动压测（终端 2）─────────────────────
cd example_application/command
APP_ENV_application_env=test go run . bench \
  --rps=6000 --duration=100s --seed=1000 --workers=512

# ── 步骤 4：采集（终端 3，压测开始 15 秒后）───────
# 为什么等 15 秒：跳过 warmup 与爬坡期
sleep 15
go tool pprof -proto -output=cpu.pb.gz \
  "http://127.0.0.1:6060/debug/pprof/profile?seconds=45"

go tool pprof -proto -output=heap_t1.pb.gz http://127.0.0.1:6060/debug/pprof/heap
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head -1   # 压测中
sleep 35
go tool pprof -proto -output=heap_t2.pb.gz http://127.0.0.1:6060/debug/pprof/heap
go tool pprof -proto -output=block.pb.gz   http://127.0.0.1:6060/debug/pprof/block
go tool pprof -proto -output=mutex.pb.gz   http://127.0.0.1:6060/debug/pprof/mutex

# ── 步骤 5：压测结束静置 30 秒后复查 ──────────────
sleep 30
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head -1
# → 应回落到接近步骤 2 的基线值

# ── 步骤 6：分析 ──────────────────────────────────
go tool pprof -top -nodecount=20 cpu.pb.gz
go tool pprof -base heap_t1.pb.gz heap_t2.pb.gz
grep '^gc ' gctrace.log | tail -20
```

### 5.2 分析顺序（本次实际走过的路径）

**① 先问「该不该信这份 profile」**

```
Duration: 40s, Total samples = 26.55s (66.37%)
```
采样率 66% → 可信。若低于 30%，先加压再说。

**② CPU：先 top（找忙），再 top -cum（找路径）**

```
      flat  flat%   sum%        cum   cum%
     7.77s 29.27% 29.27%      7.77s 29.27%  syscall.Syscall6
     4.49s 16.91% 46.18%      4.49s 16.91%  runtime.futex
     0.06s  0.23% 58.57%      3.78s 14.24%  lumberjack.(*Logger).Write   ← flat 极低，cum 14%
```

`Syscall6` + `futex` 占 46% → **系统在等，不在算**。这是往 block/mutex 追查的信号。

**③ Block：找「等待」的来源**

```
Showing nodes accounting for 13.03hrs, 67.66% of 19.27hrs total

  13.03hrs 67.66%  sync.(*Mutex).Lock (inline)
         0     0%  zerolog.(*Event).Msg
         0     0%  SyncLumberjackWriter.Write
         0     0%  lumberjack.(*Logger).Write
  12.47hrs 64.72%  fiber.(*App).handler
```

67.66% 的阻塞在**同一个日志锁**上，且栈显示它在 `fiber.(*App).handler` 内——**每个请求都同步等这把锁**。

> 注意中间几行 flat 为 0：它们只是调用链的传递节点，真正阻塞发生在 `Mutex.Lock`。**读 block/mutex profile 要看整条栈，不要只看 flat 非零的那一行。**

**④ Mutex：交叉验证**

```
  10.52hrs   100%   100%  sync.(*Mutex).Unlock
  10.05hrs 95.57%        fiberzerolog.New.func1
```

100% 的锁争用来自同一处 → 与 block profile 结论一致。**三份 profile 交叉印证，结论才可靠。**

**⑤ GC：排除嫌疑**

```
gc 161 @208.982s 0%: 0.12+1.3+0.033 ms clock, 45->46->23 MB, 46 MB goal, 16 P
```

231 次 GC 中 175 次为 0%，存活堆稳定 22–24MB。

> **本次最重要的教训**：p50 已劣化到 139ms，GC 却是 0%。**这反证了延迟问题与 GC 无关。** 若当初凭直觉去调 `GOGC`，方向完全错误。**先用 gctrace 排除 GC，再往别处找。**

**⑥ 验证修复**

只改一个变量（`asyncConf.enable: false → true`），其余配置、随机种子、负载模型完全一致：

```
              同步        异步
p50        139.5ms  →   461µs      302×
p99        175.2ms  →   2.8ms       62×
mutex 争用  10.52hrs →   464ms   ~81600×
```

异步模式的 CPU profile 中日志相关符号**完全消失**。

### 5.3 判断「这是不是问题」的三个追问

**命中 ≠ 有问题。** 热路径上的函数出现在 profile 上是必然的。

#### 追问一：cum 高是不是因为它是入口？

```bash
go tool pprof -peek='fiberzerolog.New.func1$' cpu.pb.gz
```

```
     0.08s  0.11%     26.30s 35.10%   fiberzerolog.New.func1
                      24.39s 92.74% |   fiber.(*Ctx).Next        ← 下游所有 handler
                       0.71s  2.70% |   fiberzerolog.(*Config).logger
                       0.53s  2.02% |   zerolog.(*Event).Msg
```

35.10% 里 **92.74% 是 `Ctx.Next`**——中间件自身只有约 2.2%。若直接看 35% 就下结论，会得出完全错误的判断。

#### 追问二：是一次性还是每请求？

构造函数的大额分配不是泄漏：

```
256MB 91.05%  go-diodes.NewManyToOne     ← 启动时一次性 make()，非泄漏
```

判断方法：**做差分**。

```bash
go tool pprof -base heap_t1.pb.gz heap_t2.pb.gz
```

```
Showing nodes accounting for -5344.56kB, 62.66% of 8528.94kB total
-1166.01kB  sonic/internal/encoder/vars.init.func1
   -1028kB  bufio.NewReaderSize
```

**差分为负 → 稳态下内存正被回收 → 无泄漏。**

#### 追问三：优化它能提升什么？

若系统瓶颈在 DB（`findRunnable` 22.66%），优化任何框架代码都**不会**提升吞吐。

实例：`validateID` 占 1.60%，比整个 JSON 序列化还贵（为校验单一字符串走了完整反射）。技术上可省，但**在只用 10% CPU 的系统上省 1.6% 毫无意义**，代价却是失去 validator 的统一错误信息与多语言支持。**记录它，不改它。**

### 5.4 避坑清单

**采集**
1. 用 `go tool pprof -proto -output=`，不要 `curl -o`
2. block/mutex 必须先 `SetBlockProfileRate` / `SetMutexProfileFraction`
3. `writeTimeout` 必须大于 CPU profile 的 `seconds`
4. 采集必须在**稳定负载期**，跳过 warmup 与爬坡

**分析**
5. 先看采样率，低于 30% 的 profile 不要信
6. 中间件的 cum 必须用 `-peek` 拆掉 `Next` 分支
7. heap 分「存量」（inuse）与「流量」（alloc），不要混用
8. 泄漏判断一定要做差分，不要看单次快照
9. block/mutex 的绝对值无意义，只看相对占比
10. 单次大分配（如 `make([]T, N)`）**可能被 pprof 采样完全遗漏**，改用 `runtime.MemStats`
11. 函数可能被内联而不出现，用 `-lines` 或检查是否成本低于采样精度
12. 微基准的 profile 含测试脚手架成本，优化前先确认热点归属

**环境**
13. 压测客户端与服务端同机会互相争抢 CPU，`actual rps` 达不到目标属正常
14. `dropped > 0` 时整份数据作废
15. 压测期间务必关闭控制台日志（同步写 stdout 会掩盖真实开销）

---

## 六、快速索引

```bash
# 采集（一律 -proto）
go tool pprof -proto -output=cpu.pb.gz "http://127.0.0.1:6060/debug/pprof/profile?seconds=45"
go tool pprof -proto -output=heap.pb.gz  http://127.0.0.1:6060/debug/pprof/heap
go tool pprof -proto -output=block.pb.gz http://127.0.0.1:6060/debug/pprof/block
go tool pprof -proto -output=mutex.pb.gz http://127.0.0.1:6060/debug/pprof/mutex
curl -s "http://127.0.0.1:6060/debug/pprof/goroutine?debug=1" | head -1

# 分析
go tool pprof -top -nodecount=20 cpu.pb.gz          # 找忙
go tool pprof -top -cum -nodecount=20 cpu.pb.gz     # 找路径
go tool pprof -peek='函数名$' cpu.pb.gz              # 拆解归属
go tool pprof -list='函数名' cpu.pb.gz               # 逐行
go tool pprof -focus='fiberhouse' -top cpu.pb.gz    # 只看自己的代码
go tool pprof -http=:8000 cpu.pb.gz                 # 火焰图

# 差分（判泄漏）
go tool pprof -base heap_t1.pb.gz heap_t2.pb.gz

# 分配速率（判 GC 压力）
go tool pprof -sample_index=alloc_space -top -nodecount=15 heap.pb.gz

# 单次大分配（pprof 会漏）
curl -s "http://127.0.0.1:6060/debug/pprof/heap?debug=1" | grep -E "^# (HeapAlloc|HeapInuse|Sys) "

# 微基准
go test -run='^$' -bench=. -benchmem -benchtime=2s -count=10 ./pkg/
benchstat old.txt new.txt

# GC
GODEBUG=gctrace=1 ./server 2>&1 | grep '^gc '
```
