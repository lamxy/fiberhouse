// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

// Repository 实现了RepositoryLocator接口
// 用于被具体的Repository实例继承，如UserRepository、OrderRepository等
// 主要包含服务名称、应用上下文、获取实例等基础方法
type Repository struct {
	Ctx  IContext
	name string
}

func NewRepository(ctx IApplicationContext) *Repository {
	return &Repository{
		Ctx: ctx,
	}
}

// GetName 获取名称，通常用于标记注册器名称或用于容器注册的keyName
func (r *Repository) GetName() string {
	return r.name
}

// SetName 设置名称，通常用于标记注册器名称或用于容器注册的keyName
func (r *Repository) SetName(name string) Locator {
	r.name = name
	return r
}

// GetContext 获取应用上下文
func (r *Repository) GetContext() IContext {
	return r.Ctx
}

// GetInstance 获取实例（从全局管理器获取具体的单例）
func (r *Repository) GetInstance(namespaceKey string) (interface{}, error) {
	gm := r.GetContext().GetContainer()
	return gm.Get(namespaceKey)
}
