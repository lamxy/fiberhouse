package fiberhouse

import (
	providerCtx "github.com/lamxy/fiberhouse/provider/context"
	"sync"
)

type IContextCoreWrapper interface {
	WithAppCtx(appCtx IApplicationContext) providerCtx.ICoreContext
	Release()
}

type ContextCore struct {
	ctx any
}

// 对象池
var contextCorePool = sync.Pool{
	New: func() interface{} {
		return &ContextCore{}
	},
}

// Context 从对象池获取 IContextCoreWrapper 实例
func Context(c any) *ContextCore {
	ctx := contextCorePool.Get().(*ContextCore)
	ctx.ctx = c
	return ctx
}

// WithAppCtx 接收应用上下文参数，返回核心上下文接口，内部核心上下文提供者管理器自动依据启动配置的核心参数CoreType决定返回哪种核心上下文实现JSON响应
func (c *ContextCore) WithAppCtx(appCtx IContext) providerCtx.ICoreContext {
	// 函数结束时释放回对象池
	defer c.Release()

	// 获取核心上下文管理器单例
	manager := NewCoreCtxPManagerOnce(appCtx)

	ctx, err := manager.LoadProvider(func(manager IProviderManager) (any, error) {
		return c.ctx, nil
	})
	if err != nil {
		panic(err)
	}
	coreCtx, ok := ctx.(providerCtx.ICoreContext)
	if !ok {
		panic("loaded core context provider is not ICoreContext")
	}
	return coreCtx
}

// Release 释放对象回对象池
func (c *ContextCore) Release() {
	c.ctx = nil
	contextCorePool.Put(c)
}

// CoreContext 全局函数，接收任意类型参数，返回核心上下文接口，内部核心上下文提供者管理器自动依据启动配置的核心参数CoreType决定返回哪种核心上下文实现JSON响应
func CoreContext(c any) providerCtx.ICoreContext {
	// 获取核心上下文管理器单例
	manager := NewCoreCtxPManagerParentOnce()

	ctx, err := manager.LoadProvider(func(manager IProviderManager) (any, error) {
		return c, nil
	})
	if err != nil {
		panic(err)
	}
	coreCtx, ok := ctx.(providerCtx.ICoreContext)
	if !ok {
		panic("loaded core context provider is not ICoreContext")
	}
	return coreCtx
}
