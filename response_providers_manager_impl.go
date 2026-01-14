// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"fmt"
	"github.com/lamxy/fiberhouse/response"
)

// RespInfoProtobufProvider 响应信息 Protobuf 提供者
type RespInfoProtobufProvider struct {
	IProvider
}

func NewRespInfoProtobufProvider() *RespInfoProtobufProvider {
	son := &RespInfoProtobufProvider{
		IProvider: NewProvider().SetName("application/x-protobuf").SetType(ProviderTypeDefault().GroupResponseInfoChoose),
	}
	son.MountToParent(son)
	return son
}

// Initialize 初始化
func (p *RespInfoProtobufProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	return response.GetRespInfoPB(), nil
}

// RespInfoMsgpackProvider 响应信息 Msgpack 提供者
type RespInfoMsgpackProvider struct {
	IProvider
}

func NewRespInfoMsgpackProvider() *RespInfoMsgpackProvider {
	son := &RespInfoMsgpackProvider{
		IProvider: NewProvider().SetName("application/msgpack").SetType(ProviderTypeDefault().GroupResponseInfoChoose),
	}
	son.MountToParent(son)
	return son
}

// Initialize 初始化
func (p *RespInfoMsgpackProvider) Initialize(ctx IContext, initFunc ...ProviderInitFunc) (any, error) {
	return response.GetRespInfoMsgPack(), nil
}

// PManager------------------------------------------------------------------------------------------------------------

// RespInfoPManager 响应信息提供者管理器
type RespInfoPManager struct {
	IProviderManager
}

func NewRespInfoPManager(ctx IContext) *RespInfoPManager {
	son := &RespInfoPManager{
		IProviderManager: NewProviderManager(ctx).
			SetName("RespInfoPManager").
			SetType(ProviderTypeDefault().GroupResponseInfoChoose),
		// .SetOrBindToLocation(ProviderLocationDefault().LocationResponseInfoInit, true),  // 尚未挂载子实例到父属性，此处绑定内部将父实例绑定到执行位点
	}
	// 挂载子实例到父属性，设置并绑定子实例到执行位点
	son.MountToParent(son).SetOrBindToLocation(ProviderLocationDefault().LocationResponseInfoInit, true)
	return son
}

// LoadProvider 加载提供者
func (m *RespInfoPManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	if len(loadFunc) == 0 {
		return nil, fmt.Errorf("manager '%s': no load function provided", m.Name())
	}
	anything, err := loadFunc[0](m)
	if err != nil {
		return nil, err
	}
	contentType, ok := anything.(string)
	if !ok {
		return nil, errors.New("loadFunc manager '" + m.Name() + "': expected string of http Content-Type")
	}
	return m.GetProvider(contentType)
}
