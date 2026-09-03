// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// 就绪检查（readiness）支持模块。
//
// 该模块只提供与引擎无关的就绪状态计算与 http.HandlerFunc，不自行注册任何路由：
// 框架核心不介入应用的路由布局，暴露路径、认证与可见范围应由应用决定。
// 应用可在自己的中间件 provider 中挂载，Fiber 侧可用 adaptor.HTTPHandlerFunc 适配。

package fiberhouse

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/lamxy/fiberhouse/globalmanager"
)

// ReadinessStatus 描述一次就绪检查的结果。
type ReadinessStatus struct {
	Ready bool `json:"ready"`
	// Unhealthy 列出探测失败的全局对象 key，按容器遍历顺序给出。
	Unhealthy []string `json:"unhealthy,omitempty"`
	// Errors 记录探测本身出错（而非探测结果为不健康）的 key 及原因。
	Errors map[string]string `json:"errors,omitempty"`
}

// CheckReadiness 遍历全局容器计算就绪状态。
//
// 未初始化的 lazy entry 不会被触发初始化，未实现 HealthChecker 的对象视为健康
// （沿用 GlobalManager.CheckHealth 的既有语义）。
func CheckReadiness(gm *globalmanager.GlobalManager) ReadinessStatus {
	status := ReadinessStatus{Ready: true}
	if gm == nil {
		return status
	}
	gm.Range(func(key, _ interface{}) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		healthy, err := gm.CheckHealth(name)
		if err != nil {
			if status.Errors == nil {
				status.Errors = make(map[string]string, 1)
			}
			status.Errors[name] = err.Error()
			return true
		}
		if !healthy {
			status.Ready = false
			status.Unhealthy = append(status.Unhealthy, name)
		}
		return true
	})
	return status
}

// ReadinessHandlerOption 配置就绪检查处理器。
type ReadinessHandlerOption func(*readinessHandlerOptions)

type readinessHandlerOptions struct {
	cacheTTL time.Duration
}

// WithReadinessCacheTTL 设置就绪结果的缓存时长，默认 2 秒。
//
// 探针通常由多个副本高频访问，每次请求都真实 Ping 一遍全部依赖会给数据库带来
// 可观的无效连接压力。传入 0 或负值可关闭缓存。
func WithReadinessCacheTTL(ttl time.Duration) ReadinessHandlerOption {
	return func(o *readinessHandlerOptions) {
		o.cacheTTL = ttl
	}
}

// NewReadinessHandler 构建就绪探针处理器。
//
// 就绪时返回 200，存在不健康依赖时返回 503，响应体为 ReadinessStatus 的 JSON。
// 由编排系统（K8s readinessProbe）或负载均衡器据此摘除流量。
//
// 该处理器**只应**用于 readiness，不要挂到 livenessProbe：liveness 探测外部依赖
// 会在数据库抖动时导致全体 Pod 被重启，把单点依赖故障放大成雪崩。
// liveness 只需检查进程自身存活即可。
func NewReadinessHandler(gm *globalmanager.GlobalManager, opts ...ReadinessHandlerOption) http.HandlerFunc {
	options := readinessHandlerOptions{cacheTTL: 2 * time.Second}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	var (
		mu       sync.Mutex
		cached   ReadinessStatus
		cachedAt time.Time
	)

	return func(w http.ResponseWriter, r *http.Request) {
		var status ReadinessStatus
		if options.cacheTTL > 0 {
			mu.Lock()
			if !cachedAt.IsZero() && time.Since(cachedAt) < options.cacheTTL {
				status = cached
			} else {
				status = CheckReadiness(gm)
				cached, cachedAt = status, time.Now()
			}
			mu.Unlock()
		} else {
			status = CheckReadiness(gm)
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if status.Ready {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(status)
	}
}
