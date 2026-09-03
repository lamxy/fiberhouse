// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

/*
pprof 性能剖析服务模块，基于标准库 net/http/pprof 提供独立端口的诊断服务。

# 设计说明

诊断服务不挂载到业务引擎（fiber/gin/hertz）的路由上，而是独立监听一个端口，原因：

  - 引擎无关：一份实现覆盖全部核心引擎，无需为每种引擎各写一份适配
  - 诊断可用性：业务引擎 goroutine 饱和或中间件死锁时，独立服务仍可响应，
    而这恰恰是最需要采集 profile 的时刻
  - 零业务开销：不进入业务中间件链，对正常请求路径无任何影响
  - 隔离暴露面：业务端口不暴露诊断接口，默认仅绑定回环地址

# 支持的 profile

cpu、heap、goroutine、block、mutex、allocs、threadcreate、trace 等。
其中 block 与 mutex 需要显式设置采样率才会产生数据，见 PprofConfig 说明。
*/

package fiberhouse

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strconv"
	"sync"

	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
)

// PprofServer pprof 诊断服务，管理独立 http.Server 的生命周期
type PprofServer struct {
	cfg    PprofConfig
	logger bootstrap.LoggerWrapper
	origin appconfig.LogOrigin

	mu      sync.Mutex
	server  *http.Server
	started bool
}

// NewPprofServer 依据配置创建 pprof 诊断服务实例
func NewPprofServer(cfg PprofConfig, logger bootstrap.LoggerWrapper, origin appconfig.LogOrigin) *PprofServer {
	return &PprofServer{
		cfg:    cfg,
		logger: logger,
		origin: origin,
	}
}

// Addr 返回诊断服务的监听地址
func (p *PprofServer) Addr() string {
	return net.JoinHostPort(p.cfg.Host, strconv.Itoa(p.cfg.Port))
}

// Start 启动诊断服务。
//
// 未启用时直接返回 nil，不做任何监听。启动过程中先完成端口监听（同步），
// 再在独立 goroutine 中提供服务，因此端口冲突等错误可以在此同步返回，
// 而不会被静默吞掉。
//
// 诊断服务的失败不应影响业务启动，调用方通常只记录错误而不中断流程。
func (p *PprofServer) Start() error {
	if !p.cfg.Enable {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return nil
	}

	// block 与 mutex profile 必须显式开启采样率，否则对应端点仅返回空数据
	p.applyProfileRates()

	listener, err := net.Listen("tcp", p.Addr())
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler:      p.buildHandler(),
		ReadTimeout:  p.cfg.ReadTimeout,
		WriteTimeout: p.cfg.WriteTimeout,
	}
	p.server = server
	p.started = true

	// 捕获 server 局部变量而非读取 p.server：
	// Shutdown 会将 p.server 置空，若此处再读字段会与关闭流程产生数据竞争
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil &&
			!errors.Is(serveErr, http.ErrServerClosed) {
			p.logError(serveErr, "pprof server serve failed")
		}
	}()

	p.logInfo("pprof server listening on " + p.Addr() + p.cfg.Prefix + "/")
	return nil
}

// Shutdown 优雅关闭诊断服务
func (p *PprofServer) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started || p.server == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, p.cfg.ShutdownTimeout)
	defer cancel()

	err := p.server.Shutdown(shutdownCtx)
	p.started = false
	p.server = nil

	if err != nil {
		return err
	}

	p.logInfo("pprof server shutdown complete")
	return nil
}

// applyProfileRates 应用 block 与 mutex 的采样率设置。
//
// 二者默认关闭（0），开启后存在运行时开销，应仅在诊断期间启用。
func (p *PprofServer) applyProfileRates() {
	if p.cfg.BlockRate > 0 {
		runtime.SetBlockProfileRate(p.cfg.BlockRate)
		p.logInfo("pprof block profile enabled, rate=" + strconv.Itoa(p.cfg.BlockRate) + "ns")
	}
	if p.cfg.MutexFraction > 0 {
		runtime.SetMutexProfileFraction(p.cfg.MutexFraction)
		p.logInfo("pprof mutex profile enabled, fraction=1/" + strconv.Itoa(p.cfg.MutexFraction))
	}
}

// buildHandler 构建 pprof 路由处理器
//
// pprof.Index 已覆盖 heap、goroutine、block、mutex、allocs、threadcreate 等
// 通过 runtime/pprof 注册的 profile，无需逐个显式注册。
func (p *PprofServer) buildHandler() http.Handler {
	prefix := p.cfg.Prefix
	mux := http.NewServeMux()

	mux.HandleFunc(prefix+"/", pprof.Index)
	mux.HandleFunc(prefix+"/cmdline", pprof.Cmdline)
	mux.HandleFunc(prefix+"/profile", pprof.Profile) // CPU profile
	mux.HandleFunc(prefix+"/symbol", pprof.Symbol)
	mux.HandleFunc(prefix+"/trace", pprof.Trace)

	// 访问根路径时重定向到 pprof 索引页，便于人工排查
	mux.Handle("/", http.RedirectHandler(prefix+"/", http.StatusFound))

	return p.withBasicAuth(mux)
}

// withBasicAuth 按配置为诊断服务包装 BasicAuth 鉴权。
//
// 即便默认绑定回环地址，在需要绑定 0.0.0.0 的内网场景下仍应开启鉴权。
func (p *PprofServer) withBasicAuth(next http.Handler) http.Handler {
	if !p.cfg.AuthEnable {
		return next
	}

	expectedUser := []byte(p.cfg.AuthUsername)
	expectedPass := []byte(p.cfg.AuthPassword)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		// 使用恒定时间比较，避免通过响应时间差异推断凭据
		userMatch := subtle.ConstantTimeCompare([]byte(user), expectedUser) == 1
		passMatch := subtle.ConstantTimeCompare([]byte(pass), expectedPass) == 1

		if !ok || !userMatch || !passMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="pprof"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// logInfo 以框架日志器输出信息级日志
func (p *PprofServer) logInfo(msg string) {
	if p.logger == nil {
		return
	}
	p.logger.InfoWith(p.origin).Str("component", "Pprof").Msg(msg)
}

// logError 以框架日志器输出错误级日志
func (p *PprofServer) logError(err error, msg string) {
	if p.logger == nil {
		return
	}
	p.logger.ErrorWith(p.origin).Str("component", "Pprof").Err(err).Msg(msg)
}
