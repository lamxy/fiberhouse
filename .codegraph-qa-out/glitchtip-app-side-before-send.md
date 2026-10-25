# 应用端方案（备用）：接入 GlitchTip/Sentry 时整理 FiberHouse 错误事件

状态：已在本机 GlitchTip 6.2.6 + sentry-go v0.50.0 验证有效，尚未落到任何应用。验证过程见
`glitchtip-error-field-verification.md`，可运行实现见 `glitchtip-verify/harness/appside.go`。

## 要解决的两个问题

1. **返回 error 路径的 culprit 误导**：sentryzerolog 在 writer 内部调用 `sentry.NewStacktrace()`，
   最内层几帧总是 zerolog、jsonparser、io 自身；而 sentry-go v0.50.0 把 GOROOT、vendor、third_party
   以外的帧一律标为 in-app，且没有 InAppExclude 一类的配置项。结果 culprit 显示
   `github.com/buger/jsonparser in ObjectEach`。
2. **大段字段读不了**：sentryzerolog 把 `message`、`error` 以外的字段全部转成 tag。`stack`、`reqHeaders`
   等在 GlitchTip 页面上被压成一行、侧栏统计约 200 字符截断。

两者都源于 sentryzerolog 的映射方式，框架日志结构保持与厂商无关，因此放在应用端的 `BeforeSend` 处理。

## 做法

```go
// logPipelineModules 是日志写入链路上的包：sentryzerolog 在 writer 内部抓堆栈，
// 最内层这几帧总是日志库自身，对定位问题没有帮助。
var logPipelineModules = []string{
	"github.com/rs/zerolog",
	"github.com/buger/jsonparser",
	"io",
}

// bulkyTags 是体积大、几乎每条都不同的字段，放在 tag 里既读不了也没法检索，
// 改放到 fiberhouse 上下文区块。
var bulkyTags = []string{"stack", "DebugStackLines", "reqHeaders", "reqParams", "reqQueries"}

// fiberhouseBeforeSend 返回整理 FiberHouse 错误事件的 BeforeSend。
// appModules 是应用自己的模块前缀（如 "github.com/acme/order-service"），只有它们标记为 in-app。
func fiberhouseBeforeSend(appModules ...string) func(*sentry.Event, *sentry.EventHint) *sentry.Event {
	return func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
		for i := range event.Exception {
			st := event.Exception[i].Stacktrace
			if st == nil {
				continue
			}
			st.Frames = trimLogPipeline(st.Frames)
			for j := range st.Frames {
				st.Frames[j].InApp = hasAnyPrefix(st.Frames[j].Module, appModules)
			}
		}

		moved := sentry.Context{}
		for _, key := range bulkyTags {
			if value, ok := event.Tags[key]; ok {
				moved[key] = value
				delete(event.Tags, key)
			}
		}
		if len(moved) > 0 {
			if event.Contexts == nil {
				event.Contexts = map[string]sentry.Context{}
			}
			event.Contexts["fiberhouse"] = moved
		}
		return event
	}
}

// trimLogPipeline 去掉最内层（切片末尾）连续的日志链路帧
func trimLogPipeline(frames []sentry.Frame) []sentry.Frame {
	end := len(frames)
	for end > 0 && hasAnyPrefix(frames[end-1].Module, logPipelineModules) {
		end--
	}
	return frames[:end]
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if s == p || strings.HasPrefix(s, p+"/") || strings.HasPrefix(s, p+".") {
			return true
		}
	}
	return false
}
```

接入（应用初始化 sentry 处）：

```go
sentry.Init(sentry.ClientOptions{
	Dsn:        dsn,
	BeforeSend: fiberhouseBeforeSend("github.com/acme/order-service"), // 换成应用自己的模块路径
})
writer, _ := sentryzerolog.NewWithHub(sentry.CurrentHub(), sentryzerolog.Options{}) // Levels 保持默认
fiberhouse.New(&fiberhouse.BootConfig{ /* … */ LogWriters: []io.Writer{writer}})
```

注意 sentryzerolog v0.50.0 的 `New(Config)` 会自建 client，不经过全局 `sentry.Init` 的 `BeforeSend`；
要让 `BeforeSend` 生效，用 `NewWithHub(sentry.CurrentHub(), …)`，或把 `BeforeSend` 写进 `Config.ClientOptions`。

## 验证效果（GlitchTip 6.2.6）

| 场景 | 不加 BeforeSend | 加 BeforeSend |
|---|---|---|
| panic | culprit `main in panicSite`，第三方帧也标 in-app | culprit 不变，只有应用帧标 in-app |
| 返回 error | culprit `github.com/buger/jsonparser in ObjectEach` | culprit `fiberhouse in (*ErrorHandler).DefaultStackTraceHandler` |
| tag | 含 stack、reqHeaders 等大段值 | 只剩 Msg、Origin、caller 等短字段；大段值在 `fiberhouse` 上下文区块完整显示 |

## 落地前要确认

- 应用 `go.mod` 锁定的 sentry-go 版本（只验证了 v0.50.0；`Frame.Module`、`Event.Contexts` 字段名随版本可能变化）。
- `appModules` 填应用自己的模块路径；`main` 包只适用于单文件示例。
- sentryzerolog `Options.Levels` 不要包含 Warn，否则 4xx 业务错误仍会建 issue。
- 如果框架日后改了字段名（`stack`、`reqHeaders` 等），同步更新 `bulkyTags`。
