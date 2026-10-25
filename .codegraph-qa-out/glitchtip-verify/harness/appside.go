package main

import (
	"strings"

	"github.com/getsentry/sentry-go"
)

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
