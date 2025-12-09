// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"context"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/lamxy/fiberhouse/component/jsoncodec"
	"github.com/redis/go-redis/v9"
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

// RunSync 启动任务处理器，并阻塞等待任务处理完成
func (tk *TaskWorker) RunSync() error {
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
func (tk *TaskWorker) RunAsync() (err error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				tk.GetContext().GetLogger().Error(tk.GetContext().GetConfig().LogOriginTask()).Msgf("[Asynq] Worker panic: %v", r)
			}
		}()
		_ = tk.runSimple()
	}()
	return
}

// TaskDispatcher 封装 asynq.Client，简化任务发送到 asynq 服务器的流程，支持异步和同步任务调度。
type TaskDispatcher struct {
	Client *asynq.Client
}

func NewTaskDispatcher(redisClient *redis.Client) *TaskDispatcher {
	return &TaskDispatcher{
		Client: asynq.NewClientFromRedisClient(redisClient),
	}
}

// Enqueue 将任务添加到asynq队列中
func (td *TaskDispatcher) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return td.Client.Enqueue(task, opts...)
}

// EnqueueContext 将任务添加到asynq队列中，支持上下文
func (td *TaskDispatcher) EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
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
	origin, err := gm.Get(ctx.GetStarter().GetApplication().GetFastJsonCodecKey())
	if err != nil {
		return nil, err
	}
	if instance, ok := origin.(JsonWrapper); ok {
		return instance, nil
	}
	return nil, fmt.Errorf("assertion failure for type of '%s' instance", ctx.GetStarter().GetApplication().GetFastJsonCodecKey())
}

// GetMustJsonHandler 确保获取到的JSON处理器不为nil，如果获取失败，则返回默认的JSON处理器
func (p *PayloadBase) GetMustJsonHandler(ctx IContext) JsonWrapper {
	if ctx == nil {
		return p.GetDefault(ctx)
	}
	gm := ctx.GetContainer()
	origin, err := gm.Get(ctx.GetStarter().GetApplication().GetFastJsonCodecKey())
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
