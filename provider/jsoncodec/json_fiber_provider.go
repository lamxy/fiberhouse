package jsoncodec

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/jsoncodec"
)

type JsonJCodecFiberProvider struct {
	fiberhouse.IProvider
}

// NewJsonJCodecFiberProvider 创建一个新的 JSON 编解码提供者
func NewJsonJCodecFiberProvider() *JsonJCodecFiberProvider {
	return &JsonJCodecFiberProvider{
		IProvider: fiberhouse.NewProvider().SetName("json_codec").SetTarget("Fiber").SetType(fiberhouse.ProviderTypeDefault().GroupJsonCodecChoose),
	}
}

// Initialize 初始化 JSON 编解码提供者
func (j *JsonJCodecFiberProvider) Initialize(ctx fiberhouse.IContext, fn ...fiberhouse.ProviderInitFunc) (any, error) {
	j.Check()
	if j.Status() == fiberhouse.StateLoaded {
		return nil, nil
	}
	// 实现 JSON 编解码器的注册逻辑
	jcodec := jsoncodec.StdJsonDefault()

	j.SetStatus(fiberhouse.StateLoaded)
	return jcodec, nil
}
