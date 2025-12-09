// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

// Locator 定位器接口，定义了获取上下文、名称、实例等方法
// 以及错误恢复方法。用于分层和管理应用中的业务组件或服务实例。
// 该接口可以被具体的API、Service、Repository等定位器实现。
type Locator interface {
	// 获取全局上下文对象
	GetContext() IContext
	// 获取定位器名称空间
	GetName() string
	// 设置定位器名称空间
	SetName(string) Locator // replace interface{}
	// GetInstance 获取实例（从全局管理器获取具体的单例）
	GetInstance(string) (interface{}, error)
}

// ApiLocator Api层定位器
type ApiLocator = Locator

// ServiceLocator 服务层定位器
type ServiceLocator = Locator

// RepositoryLocator 仓储层定位器
type RepositoryLocator = Locator
