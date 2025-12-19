package jsoncodec

import (
	"github.com/lamxy/fiberhouse"
)

type SonicJCodecFiberProvider struct {
	fiberhouse.IProvider
}

// NewJCodecProvider 创建一个新的 JSON 编解码提供者
func NewSonicJCodecFiberProvider() *SonicJCodecFiberProvider {
	return &SonicJCodecFiberProvider{
		IProvider: fiberhouse.NewProvider().SetName("sonic_json_codec").SetTarget("fiber").SetType(fiberhouse.ProviderTypeDefault().GroupJsonCodecChoose),
	}
}

// Initialize 初始化 JSON 编解码提供者
func (j *SonicJCodecFiberProvider) Initialize(ctx fiberhouse.IContext, fn ...fiberhouse.ProviderInitFunc) (any, error) {
	j.Check()
	if j.Status() == fiberhouse.StateLoaded {
		return nil, nil
	}
	// 实现 JSON 编解码器的注册逻辑
	jcodec, err := fiberhouse.GetInstance[fiberhouse.JsonWrapper](ctx.GetStarter().GetApplication().GetDefaultJsonCodecKey())
	if err != nil {
		return nil, err
	}
	j.SetStatus(fiberhouse.StateLoaded)
	return jcodec, nil
}
