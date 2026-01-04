package fiberhouse

import (
	"github.com/lamxy/fiberhouse/component/jsoncodec"
	"github.com/lamxy/fiberhouse/constant"
)

type JsonJCodecFiberProvider struct {
	IProvider
}

// NewJsonJCodecFiberProvider 创建一个新的 JSON 编解码提供者
func NewJsonJCodecFiberProvider() *JsonJCodecFiberProvider {
	return &JsonJCodecFiberProvider{
		IProvider: NewProvider().
			SetName("JsonJCodecFiberProvider").
			SetVersion(constant.TrafficCodecWithStd).
			SetTarget(constant.CoreTypeWithFiber).
			SetType(ProviderTypeDefault().GroupTrafficCodecChoose),
	}
}

// Initialize 重载初始化 JSON 编解码提供者
func (j *JsonJCodecFiberProvider) Initialize(ctx IContext, fn ...ProviderInitFunc) (any, error) {
	j.Check()
	if j.Status() == StateLoaded {
		return nil, nil
	}
	// 实现 JSON 编解码器的注册逻辑
	jcodec := jsoncodec.StdJsonDefault()

	j.SetStatus(StateLoaded)
	return jcodec, nil
}
