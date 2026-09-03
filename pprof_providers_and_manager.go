// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

/*
pprof 诊断服务的提供者与管理器，将 PprofServer 接入框架的位点生命周期。

# 位点编排

  - LocationServerRunBefore  : 启动诊断服务（业务引擎开始监听之前）
  - LocationServerShutdownAfter: 关闭诊断服务（业务引擎关闭之后）

两个位点均通过 LoadProviderManagersAtLocation 统一分派，支持注册多个管理器。

# 引擎无关

诊断服务独立监听端口，与核心引擎类型无关，因此提供者类型使用
GroupProviderAutoRun（自动运行组，不做 Target 匹配约束）。
*/

package fiberhouse

import (
	"context"
	"sync"

	"github.com/lamxy/fiberhouse/appconfig"
)

// pprofTargetAny 诊断服务的目标标识。
//
// 诊断服务独立监听端口，与核心引擎类型无关，加载时不做 Target 匹配约束，
// 此处仅用于标识语义。
const pprofTargetAny = "any"

// pprof 启动与关闭使用各自独立的提供者类型。
//
// 框架在 resolveManagerWithProviders 中按「类型 ID 相等」把提供者分派给管理器，
// 且首个匹配即 break。若启动与关闭提供者共用同一类型，二者会被归入同一个管理器，
// 导致关闭提供者在启动位点被执行。因此必须为两者注册独立的自定义类型。
var (
	pprofStartType    IProviderType
	pprofShutdownType IProviderType
	pprofTypeOnce     sync.Once
)

// pprofTypes 获取（并惰性注册）pprof 启动与关闭的提供者类型
func pprofTypes() (start IProviderType, shutdown IProviderType) {
	pprofTypeOnce.Do(func() {
		registry := ProviderTypeGen()
		pprofStartType = registry.MustCustom("PprofStartType")
		pprofShutdownType = registry.MustCustom("PprofShutdownType")
	})
	return pprofStartType, pprofShutdownType
}

// pprofServerInstance 进程内共享的诊断服务实例。
//
// 启动与关闭分处两个位点、由两个提供者分别驱动，需共享同一实例才能正确关闭。
var (
	pprofServerInstance *PprofServer
	pprofServerMu       sync.Mutex
)

// resolvePprofServer 获取或创建进程内唯一的诊断服务实例
func resolvePprofServer(ctx IApplicationContext) *PprofServer {
	pprofServerMu.Lock()
	defer pprofServerMu.Unlock()

	if pprofServerInstance != nil {
		return pprofServerInstance
	}

	cfg := ctx.GetConfig()
	pprofServerInstance = NewPprofServer(
		LoadPprofConfig(cfg),
		ctx.GetLogger(),
		resolvePprofLogOrigin(cfg),
	)
	return pprofServerInstance
}

// resolvePprofLogOrigin 解析诊断服务使用的日志来源标识
func resolvePprofLogOrigin(cfg appconfig.IAppConfig) appconfig.LogOrigin {
	return cfg.LogOriginFrame()
}

/*
	启动提供者
*/

// PprofStartProvider 在服务运行前启动 pprof 诊断服务
type PprofStartProvider struct {
	IProvider
}

// NewPprofStartProvider 创建 pprof 诊断服务启动提供者
func NewPprofStartProvider() *PprofStartProvider {
	startType, _ := pprofTypes()
	son := &PprofStartProvider{
		IProvider: NewProvider().
			SetName("PprofStartProvider").
			SetTarget(pprofTargetAny).
			SetType(startType),
	}
	son.MountToParent(son)
	return son
}

// Initialize 启动诊断服务。
//
// 未启用时直接返回，不产生任何监听与开销。
// 诊断服务启动失败只记录日志，不阻断业务启动流程。
func (p *PprofStartProvider) Initialize(ctx IContext, _ ...ProviderInitFunc) (any, error) {
	appCtx, ok := ctx.(IApplicationContext)
	if !ok {
		return nil, nil
	}

	server := resolvePprofServer(appCtx)
	if !server.cfg.Enable {
		return nil, nil
	}

	if err := server.Start(); err != nil {
		server.logError(err, "pprof server start failed")
	}
	return nil, nil
}

/*
	关闭提供者
*/

// PprofShutdownProvider 在服务关闭后停止 pprof 诊断服务
type PprofShutdownProvider struct {
	IProvider
}

// NewPprofShutdownProvider 创建 pprof 诊断服务关闭提供者
func NewPprofShutdownProvider() *PprofShutdownProvider {
	_, shutdownType := pprofTypes()
	son := &PprofShutdownProvider{
		IProvider: NewProvider().
			SetName("PprofShutdownProvider").
			SetTarget(pprofTargetAny).
			SetType(shutdownType),
	}
	son.MountToParent(son)
	return son
}

// Initialize 关闭诊断服务
func (p *PprofShutdownProvider) Initialize(ctx IContext, _ ...ProviderInitFunc) (any, error) {
	appCtx, ok := ctx.(IApplicationContext)
	if !ok {
		return nil, nil
	}

	server := resolvePprofServer(appCtx)
	if !server.cfg.Enable {
		return nil, nil
	}

	if err := server.Shutdown(context.Background()); err != nil {
		server.logError(err, "pprof server shutdown failed")
	}
	return nil, nil
}

/*
	管理器
*/

// PprofStartPManager pprof 启动管理器，绑定到服务运行前位点
type PprofStartPManager struct {
	IProviderManager
}

// NewPprofStartPManager 创建 pprof 启动管理器
func NewPprofStartPManager(ctx IContext) *PprofStartPManager {
	startType, _ := pprofTypes()
	son := &PprofStartPManager{
		IProviderManager: NewProviderManager(ctx).
			SetName("PprofStartPManager").
			SetType(startType),
	}
	son.MountToParent(son)
	son.SetOrBindToLocation(ProviderLocationDefault().LocationServerRunBefore, true)
	return son
}

// LoadProvider 初始化管理器内注册的 pprof 提供者
func (m *PprofStartPManager) LoadProvider(_ ...ProviderLoadFunc) (any, error) {
	m.Check()
	return loadPprofProviders(m)
}

// PprofShutdownPManager pprof 关闭管理器，绑定到服务关闭后位点
type PprofShutdownPManager struct {
	IProviderManager
}

// NewPprofShutdownPManager 创建 pprof 关闭管理器
func NewPprofShutdownPManager(ctx IContext) *PprofShutdownPManager {
	_, shutdownType := pprofTypes()
	son := &PprofShutdownPManager{
		IProviderManager: NewProviderManager(ctx).
			SetName("PprofShutdownPManager").
			SetType(shutdownType),
	}
	son.MountToParent(son)
	son.SetOrBindToLocation(ProviderLocationDefault().LocationServerShutdownAfter, true)
	return son
}

// LoadProvider 初始化管理器内注册的 pprof 提供者
func (m *PprofShutdownPManager) LoadProvider(_ ...ProviderLoadFunc) (any, error) {
	m.Check()
	return loadPprofProviders(m)
}

// loadPprofProviders 初始化管理器内全部提供者的公共逻辑。
//
// 诊断服务不依赖核心引擎实例，因此 initFunc 返回 nil 即可。
func loadPprofProviders(m IProviderManager) (any, error) {
	for _, provider := range m.List() {
		if _, err := m.InitializeProvider(provider, func(IProvider) (any, error) {
			return nil, nil
		}); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
