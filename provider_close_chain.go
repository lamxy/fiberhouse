// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"context"
	"reflect"
	"time"
)

// closeProvidersTimeout 关闭 provider 的总预算。
//
// 与 Gin 引擎关闭所用的 30 秒对齐：provider 关闭发生在引擎已停止之后，此时不再有
// 在途请求，正常情况应在毫秒级完成；这个预算只用于兜住实现不当的阻塞式 Close，
// 避免它无限期拖住整个关闭流程。
const closeProvidersTimeout = 30 * time.Second

// shutdownChainLocations 关闭链消费的三个位点。
//
// 只有绑定到这些位点的管理器才参与 provider 关闭。传给 Shutdown 的管理器集合里
// 同时混有运行位点的管理器（boot.go 的 resolveRunAndShutdownManagers 把
// before/main/after 三组拼在一起传入，而核心的运行链同样会收到运行位点管理器），
// 关闭运行位点的 provider 会误伤仍在使用的资源。
func shutdownChainLocations() [3]uint8 {
	defaults := ProviderLocationDefault()
	return [3]uint8{
		defaults.LocationServerShutdownBefore.GetLocationID(),
		defaults.LocationServerShutdown.GetLocationID(),
		defaults.LocationServerShutdownAfter.GetLocationID(),
	}
}

// closeShutdownChainProviders 关闭关闭链上各管理器持有的 provider 资源。
//
// 调用位置夹在两端之间，两端都是硬约束：
//
//   - ServerShutdownAfter 位点**之后**：该位点的 provider 仍可能使用这些资源。
//   - 全局对象释放与日志器关闭**之前**：关闭失败要靠框架日志器逐个上报，
//     日志器已关则这些错误静默丢失。
//
// 关闭失败只上报不返回：关闭链走到这一步时，引擎已停、位点已消费完毕，把错误
// 上抛只会让调用方跳过后续的全局对象释放——那是比单个 provider 关不掉严重得多的
// 泄漏。这与 releaseApplicationGlobals 对容器释放失败的处理一致。
func closeShutdownChainProviders(ctx IApplicationContext, managers []IProviderManager) {
	if ctx == nil || len(managers) == 0 {
		return
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), closeProvidersTimeout)
	defer cancel()

	locations := shutdownChainLocations()
	for _, manager := range managers {
		if manager == nil || manager.Location() == nil {
			continue
		}

		locationID := manager.Location().GetLocationID()
		onShutdownChain := false
		for _, shutdownLocation := range locations {
			if locationID == shutdownLocation {
				onShutdownChain = true
				break
			}
		}
		if !onShutdownChain || !canCloseProviders(manager) {
			continue
		}

		if err := manager.CloseProviders(closeCtx); err != nil {
			reportProviderCloseFailure(ctx, err)
		}
	}
}

// canCloseProviders 判断管理器能否安全地服务 CloseProviders 调用。
//
// 管理器惯用「嵌入 IProviderManager 接口 + 只覆写所需方法」的组合方式扩展
// （DefaultPManager、PprofStartPManager 等都是如此）。若嵌入的委托为 nil，未被
// 覆写的方法就会分派到 nil 接口上——CloseProviders 正是这样一个方法，调用它会
// panic 并炸掉整条关闭链。
//
// 这不是只为测试替身兜底：使用方自定义的管理器同样可能只覆写 Type/Location/
// LoadProvider 而不设置嵌入值。关闭路径必须对此免疫——一个装配不全的管理器
// 不应导致其余资源全部泄漏。
func canCloseProviders(manager IProviderManager) bool {
	value := reflect.ValueOf(manager)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		// 非指针实现（或 nil 指针）无法内省嵌入字段；交由调用方直接尝试。
		return value.IsValid() && !value.IsZero()
	}

	elem := value.Elem()
	if elem.Kind() != reflect.Struct {
		return true
	}

	// 找到嵌入的 IProviderManager 字段，为 nil 则说明委托缺失。
	managerType := reflect.TypeOf((*IProviderManager)(nil)).Elem()
	for i := 0; i < elem.NumField(); i++ {
		field := elem.Type().Field(i)
		if field.Anonymous && field.Type == managerType {
			return !elem.Field(i).IsNil()
		}
	}

	return true
}

// reportProviderCloseFailure 把 provider 关闭失败写入框架日志器。
//
// 全程 nil 防护：关闭路径可能在上下文尚未完全装配时被触发。
func reportProviderCloseFailure(ctx IApplicationContext, err error) {
	logger, cfg := ctx.GetLogger(), ctx.GetConfig()
	if logger == nil || cfg == nil {
		return
	}
	logger.ErrorWith(cfg.LogOriginFrame()).Err(err).
		Msg("failed to close providers during shutdown")
}
