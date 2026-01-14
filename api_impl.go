// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

// Api 实现了ApiLocator接口，相当于MVC的Controller层
// 用于被具体的Api处理器实例继承，如UserApi、ProductApi等
// 主要包含服务名称、应用上下文、获取实例等基础方法
type Api struct {
	Ctx  IContext
	name string
}

func NewApi(ctx IApplicationContext) *Api {
	return &Api{
		Ctx: ctx,
	}
}

// GetName 获取Api名称，通常用于标记注册器名称或用于容器注册的keyName
func (c *Api) GetName() string {
	return c.name
}

// SetName 设置Api名称，通常用于标记注册器名称或用于容器注册的keyName
func (c *Api) SetName(name string) Locator {
	c.name = name
	return c
}

// GetAppContext 获取应用上下文
func (c *Api) GetContext() IContext {
	return c.Ctx
}

// GetInstance 获取实例（从全局管理器获取具体的单例）
func (c *Api) GetInstance(namespaceKey string) (interface{}, error) {
	gm := c.GetContext().GetContainer()
	return gm.Get(namespaceKey)
}
