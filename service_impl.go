// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

// Service 实现了ServiceLocator接口
// 用于被具体的Service实例继承，如UserService等
// 主要包含服务名称、应用上下文、获取实例等基础方法
type Service struct {
	Ctx  IContext
	name string
}

func NewService(ctx IContext) *Service {
	return &Service{
		Ctx: ctx,
	}
}

// GetName 获取服务名称，通常用于标记注册器名称或用于容器注册的keyName
func (s *Service) GetName() string {
	return s.name
}

// SetName 设置服务名称，通常用于标记注册器名称或用于容器注册的keyName
func (s *Service) SetName(name string) Locator {
	s.name = name
	return s
}

// GetContext 获取应用上下文
func (s *Service) GetContext() IContext {
	return s.Ctx
}

// GetInstance 获取实例（从全局管理器获取具体的单例）
func (s *Service) GetInstance(namespaceKey string) (interface{}, error) {
	gm := s.GetContext().GetContainer()
	return gm.Get(namespaceKey)
}
