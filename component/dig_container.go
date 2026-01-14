// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package component

import (
	"go.uber.org/dig"
	"sync"
)

var (
	digContainer *DigContainer
	digOnce      sync.Once
)

/*
DigContainer 封装dig容器，支持连缀操作，用于命令应用或web应用启动阶段实例化对象时解决依赖注入问题，不推荐web运行时使用。
//
// e.g.

// 泛型实例包装器，实现了Set和Get方法
wrap := component.NewWrap[*OrderRepository]()

// 获取单例容器，连缀操作提供依赖构造器注册
dc := component.Container().

	Provide(func() Xxx.IApplicationContext { return ctx }).  // 依赖ctx，匿名函数注入ctx依赖
	Provide(model.NewOrderModel).
	Provide(common-module.NewCommonModel)

// 验证依赖注入是否有错误

	if dc.GetErrorCount() > 0 {
		return nil, fmt.Errorf("dig container init error: %v", dc.GetProvideErrs())
	}

// 泛型函数唤醒泛型类型实例
err := component.Invoke[*OrderRepository](warp)

// 最终通过wrap的Get方法获取唤醒的实例
instance := wrap.Get()

// ==== or ===================================================
// 非泛型方式，直接Invoke在回调函数中获取实例并执行处理逻辑

	err := dc.Invoke(func(or *OrderRepository) error {
		err := or.DoSth()  // 执行处理逻辑
		return err
	})
*/
type DigContainer struct {
	container *dig.Container
	errs      []error
}

type Wrap[T any] struct {
	target T
}

// NewWrap 获取泛型包装器，用于Invoke唤醒泛型类型的实例
func NewWrap[T any]() *Wrap[T] {
	var zero T
	return &Wrap[T]{
		target: zero,
	}
}

func (w *Wrap[T]) Set(t T) {
	w.target = t
}

func (w *Wrap[T]) Get() T {
	return w.target
}

// NewDigContainer 获取全新的依赖注入器
func NewDigContainer() *DigContainer {
	return &DigContainer{
		container: dig.New(),
		errs:      make([]error, 0),
	}
}

// NewDigContainerOnce 获取dig依赖注入器单例
func NewDigContainerOnce() *DigContainer {
	digOnce.Do(func() {
		digContainer = &DigContainer{
			container: dig.New(),
			errs:      make([]error, 0),
		}
	})
	return digContainer
}

// Provide 注入依赖，constructor为函数类型构造器，注入对象，使用函数封装返回对象
func (dc *DigContainer) Provide(constructor interface{}, opts ...dig.ProvideOption) *DigContainer {
	err := dc.container.Provide(constructor, opts...)
	if err != nil {
		dc.errs = append(dc.errs, err)
	}
	return dc
}

// GetProvideErrs 获取依赖注入错误
func (dc *DigContainer) GetProvideErrs() []error {
	return dc.errs
}

// GetErrorCount 获取错误数
func (dc *DigContainer) GetErrorCount() int32 {
	return int32(len(dc.errs))
}

// Invoke 唤醒目标类型的对象
func (dc *DigContainer) Invoke(function interface{}, opts ...dig.InvokeOption) error {
	return dc.container.Invoke(function, opts...)
}

// Container 获取单例容器
func Container() *DigContainer {
	return NewDigContainerOnce()
}

// Invoke 泛型函数，唤醒泛型类型的实例
func Invoke[T any](wrap *Wrap[T], opts ...dig.InvokeOption) error {
	dc := NewDigContainerOnce()
	fn := func(o T) error {
		wrap.Set(o)
		return nil
	}
	return dc.container.Invoke(fn, opts...)
}

// ResetDigContainer 允许重置并重建，非并发安全
func ResetDigContainer() {
	digContainer.container = nil
	digContainer.errs = nil
	digOnce = sync.Once{}
}
