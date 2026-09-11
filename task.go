// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse/component/codec/json"
	"github.com/redis/go-redis/v9"
)

// 异步任务的生命周期哨兵错误。
//
// 关闭之后继续使用是调用方的编排问题，错误必须点明「已关闭」这件事——把调用
// 直接转发给已关闭的 asynq 客户端，得到的错误与生命周期无关，调用方无从判断
// 是配置问题还是关闭顺序问题。
var (
	// ErrTaskDispatcherClosed 表示 dispatcher 已关闭，不再接受入队。
	ErrTaskDispatcherClosed = errors.New("task dispatcher is closed")
	// ErrTaskWorkerClosed 表示 worker 已关闭，不能再启动。
	ErrTaskWorkerClosed = errors.New("task worker is closed")
)

// TaskHandlerMap 定义任务处理函数映射，键为任务类型，值为对应的处理函数
type TaskHandlerMap = map[string]func(context.Context, *asynq.Task) error

// ContextKey 用于在context.Context中存储和检索应用上下文对象
type ContextKey string

// TaskWorker 是一个异步任务处理器，使用asynq库来处理任务队列
type TaskWorker struct {
	Ctx    IContext
	server *asynq.Server
	mux    *asynq.ServeMux
	// closed 保证 server.Shutdown() 只执行一次。
	//
	// 这不是优化而是必需：asynq.Server.Shutdown() 不可重入，重复调用会 panic。
	// 而容器允许同一对象以别名注册在多个 key 下，ReleaseAll 因此会多次调用
	// Close——幂等守卫必须由 wrapper 提供。
	closed atomic.Bool
}

const (
	// ContextKeyAppCtx 用于在context.Context中存储应用上下文对象的键
	ContextKeyAppCtx ContextKey = "AppContext"
)

func NewTaskWorker(appCtx IContext, redisClient *redis.Client, cfg asynq.Config) *TaskWorker {
	sm := asynq.NewServeMux()
	// 注册自定义中间件，注入项目应用上下文对象到context.Context上下文
	sm.Use(func(h asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			// 注入应用上下文
			ctxWithAppCtx := context.WithValue(ctx, ContextKeyAppCtx, appCtx)
			err := h.ProcessTask(ctxWithAppCtx, t)
			if err != nil {
				return err
			}
			return nil
		})
	})
	return &TaskWorker{
		Ctx:    appCtx,
		server: asynq.NewServerFromRedisClient(redisClient, cfg),
		mux:    sm,
	}
}

// GetContext 获取应用上下文对象
func (tk *TaskWorker) GetContext() IContext {
	return tk.Ctx
}

// GetMux 获取ServeMux引用，方便注册更多中间件
func (tk *TaskWorker) GetMux() *asynq.ServeMux {
	return tk.mux
}

// GetServer 获取Server引用
func (tk *TaskWorker) GetServer() *asynq.Server {
	return tk.server
}

// HandleFunc 注册一个处理函数，用于处理特定模式的任务
func (tk *TaskWorker) HandleFunc(pattern string, handler func(context.Context, *asynq.Task) error) {
	tk.mux.HandleFunc(pattern, handler)
}

// Handle 注册一个asynq.Handler，用于处理特定模式的任务
func (tk *TaskWorker) Handle(pattern string, handler asynq.Handler) {
	tk.mux.Handle(pattern, handler)
}

// RegisterHandlers 注册一组处理函数，用于处理特定模式的任务
func (tk *TaskWorker) RegisterHandlers(handlers TaskHandlerMap) {
	for pattern, handler := range handlers {
		tk.mux.HandleFunc(pattern, handler)
	}
}

// isClosed 报告 worker 是否已关闭
func (tk *TaskWorker) isClosed() bool {
	return tk.closed.Load()
}

// Close 停止任务处理器，实现 globalmanager.Closable。
//
// 委派给 asynq.Server.Shutdown()：它等待在途 handler 完成后才停止接收，因此
// 关闭是优雅的而非立即中断。asynq 的 Shutdown 不返回错误，故本方法恒返回 nil；
// 保留 error 返回值是为了满足 Closable 契约。
//
// 幂等：以 CAS 保证底层 Shutdown 只执行一次。asynq.Server.Shutdown() 不可重入，
// 而容器的别名注册会让 ReleaseAll 多次调用本方法。
func (tk *TaskWorker) Close() error {
	if !tk.closed.CompareAndSwap(false, true) {
		return nil
	}
	tk.server.Shutdown()
	return nil
}

// RunSync 启动任务处理器，并阻塞等待任务处理完成
func (tk *TaskWorker) RunSync() error {
	if tk.isClosed() {
		return fmt.Errorf("[Asynq] cannot start worker: %w", ErrTaskWorkerClosed)
	}
	defer func() {
		if r := recover(); r != nil {
			tk.GetContext().GetLogger().Error(tk.GetContext().GetConfig().LogOriginTask()).Msgf("[Asynq] Worker panic: %v", r)
		}
	}()
	return tk.runSimple()
}

// RunControl 启动任务处理器，并监听系统信号以便优雅地关闭服务器
func (tk *TaskWorker) runSimple() error {
	tk.GetContext().GetLogger().Info(tk.GetContext().GetConfig().LogOriginTask()).Msg("[Asynq] Staring server...")
	if err := tk.server.Run(tk.mux); err != nil {
		tk.GetContext().GetLogger().Error(tk.GetContext().GetConfig().LogOriginTask()).Err(err).Msg("[Asynq] Staring server failed")
		return err
	}
	return nil
}

// RunServer 本身支持信号监听，同时阻塞在等待信号上，因此非独立执行worker服务的条件下应置于异步go routine中启动
func (tk *TaskWorker) RunServer(sync ...bool) {
	if len(sync) > 0 && sync[0] {
		_ = tk.RunSync()
	} else {
		_ = tk.RunAsync()
	}
}

// RunAsync 启动任务处理器，并在后台运行
//
// 行为变更：不再恒返回 nil。此前具名返回值 err 从未被赋值，启动失败只写进日志，
// 调用方拿到 nil——「worker 没起来」这件事在调用侧完全不可见。
//
// 本方法现在同步校验可启动性并返回错误。后台 goroutine 内部的运行期失败仍只能
// 记日志（调用方此时已经拿到返回值），这是异步启动的固有边界；需要观测运行期
// 失败请改用 RunSync，或经 GetServer() 自建协议。
func (tk *TaskWorker) RunAsync() error {
	if tk.isClosed() {
		return fmt.Errorf("[Asynq] cannot start worker: %w", ErrTaskWorkerClosed)
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				tk.GetContext().GetLogger().Error(tk.GetContext().GetConfig().LogOriginTask()).Msgf("[Asynq] Worker panic: %v", r)
			}
		}()
		_ = tk.runSimple()
	}()
	return nil
}

// TaskDispatcher 封装 asynq.Client，简化任务发送到 asynq 服务器的流程，支持异步和同步任务调度。
type TaskDispatcher struct {
	Client *asynq.Client
	// closed 保证内部 client 只被关闭一次，并作为入队守卫的依据。
	closed atomic.Bool
}

func NewTaskDispatcher(redisClient *redis.Client) *TaskDispatcher {
	return &TaskDispatcher{
		Client: asynq.NewClientFromRedisClient(redisClient),
	}
}

// isClosed 报告 dispatcher 是否已关闭
func (td *TaskDispatcher) isClosed() bool {
	return td.closed.Load()
}

// Close 停止接受入队并释放 dispatcher 自身持有的资源，实现 globalmanager.Closable。
//
// 此前 dispatcher 没有 wrapper 级关闭方法：调用方无从表达「不再入队」，关闭链上
// 也没有它的位置——这是异步任务能力的晋级阻塞项之一。
//
// # 连接所有权：Redis 连接不由本方法关闭
//
// NewTaskDispatcher 走 asynq.NewClientFromRedisClient，即连接由调用方注入。asynq
// 对这种客户端拒绝关闭连接，其 Close() 恒定返回「redis connection is shared so
// the Client can't be closed through asynq」——这是 asynq 的所有权契约而非故障：
// 它不关闭自己不拥有的连接。同一个 *redis.Client 通常还供缓存与 worker 使用，
// 在此处关闭会拖垮其他消费者。
//
// 因此本方法把这条拒绝视为成功：dispatcher 确实没有独占资源需要释放，真正的
// 状态变更是「此后拒绝入队」。Redis 连接的回收仍由其所有者负责（容器中注册该
// client 的那个 key，或应用自身）。其他错误照常返回。
//
// 幂等：以 CAS 保证底层 Close 只执行一次，容器的别名注册会让 ReleaseAll 多次
// 调用本方法。
func (td *TaskDispatcher) Close() error {
	if !td.closed.CompareAndSwap(false, true) {
		return nil
	}
	if err := td.Client.Close(); err != nil && !isSharedRedisConnectionRefusal(err) {
		return err
	}
	return nil
}

// isSharedRedisConnectionRefusal 判断错误是否为 asynq 对共享连接的关闭拒绝。
//
// asynq 以无类型的 errors.New 返回它（client.go），没有可供 errors.Is 匹配的
// 哨兵，也没有独立类型，因此只能比对文本。匹配失败的后果是保守的：错误照常
// 上抛，表现为关闭失败上报，而不是静默吞掉真实故障。
func isSharedRedisConnectionRefusal(err error) bool {
	return err != nil && strings.Contains(err.Error(), "redis connection is shared")
}

// Enqueue 将任务添加到asynq队列中
func (td *TaskDispatcher) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if td.isClosed() {
		return nil, fmt.Errorf("[Asynq] cannot enqueue task: %w", ErrTaskDispatcherClosed)
	}
	return td.Client.Enqueue(task, opts...)
}

// EnqueueContext 将任务添加到asynq队列中，支持上下文
func (td *TaskDispatcher) EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if td.isClosed() {
		return nil, fmt.Errorf("[Asynq] cannot enqueue task: %w", ErrTaskDispatcherClosed)
	}
	return td.Client.EnqueueContext(ctx, task, opts...)
}

// IPayload 定义了获取JSON编解码器的方法接口，适用于需要处理JSON数据的场景。
type IPayload interface {
	GetJsonHandler(ctx IContext) (JsonWrapper, error)
	GetMustJsonHandler(ctx IContext) JsonWrapper
	GetDefault(ctx IContext) JsonWrapper
}

// PayloadBase 是基础负载处理器，实现 IPayload 接口，用于被实际负载对象继承，以及方便获取json编解码器
type PayloadBase struct{}

func NewPayloadBase() IPayload {
	return &PayloadBase{}
}

// GetJsonHandler 根据上下文获取JSON处理器，如果上下文为nil，则返回默认的JSON处理器
func (p *PayloadBase) GetJsonHandler(ctx IContext) (JsonWrapper, error) {
	if ctx == nil {
		return p.GetDefault(ctx), nil
	}
	gm := ctx.GetContainer()
	origin, err := gm.Get(ctx.GetStarter().GetApplication().GetFastTrafficCodecKey())
	if err != nil {
		return nil, err
	}
	if instance, ok := origin.(JsonWrapper); ok {
		return instance, nil
	}
	return nil, fmt.Errorf("assertion failure for type of '%s' instance", ctx.GetStarter().GetApplication().GetFastTrafficCodecKey())
}

// GetMustJsonHandler 确保获取到的JSON处理器不为nil，如果获取失败，则返回默认的JSON处理器
func (p *PayloadBase) GetMustJsonHandler(ctx IContext) JsonWrapper {
	if ctx == nil {
		return p.GetDefault(ctx)
	}
	gm := ctx.GetContainer()
	origin, err := gm.Get(ctx.GetStarter().GetApplication().GetFastTrafficCodecKey())
	if err != nil {
		ctx.GetLogger().Warn(ctx.GetConfig().LogOriginWeb()).Err(err).Msg("GetMustJsonHandler: GetInstance failed, returns the newly created instance.")
		return p.GetDefault(ctx)
	}
	if instance, ok := origin.(JsonWrapper); ok {
		return instance
	}
	ctx.GetLogger().Warn(ctx.GetConfig().LogOriginWeb()).Err(err).Msg("GetMustJsonHandler: GetInstance failed, returns the newly created instance.")
	return p.GetDefault(ctx)
}

// GetDefault 返回默认的JSON处理器实例
func (p *PayloadBase) GetDefault(ctx IContext) JsonWrapper {
	return jsoncodec.SonicJsonFastest()
}
