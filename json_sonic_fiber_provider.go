package fiberhouse

import "github.com/lamxy/fiberhouse/constant"

type SonicJCodecFiberProvider struct {
	IProvider
}

// NewJCodecProvider 创建一个新的 JSON 编解码提供者
func NewSonicJCodecFiberProvider() *SonicJCodecFiberProvider {
	return &SonicJCodecFiberProvider{
		IProvider: NewProvider().
			SetName("SonicJCodecFiberProvider").
			SetVersion(constant.TrafficCodecWithSonic).
			SetTarget(constant.CoreTypeWithFiber).
			SetType(ProviderTypeDefault().GroupTrafficCodecChoose),
	}
}

// Initialize 重载初始化 JSON 编解码提供者
func (j *SonicJCodecFiberProvider) Initialize(ctx IContext, fn ...ProviderInitFunc) (any, error) {
	j.Check()
	if j.Status() == StateLoaded {
		return nil, nil
	}
	// 实现 JSON 编解码器的注册逻辑
	jcodec, err := GetInstance[JsonWrapper](ctx.GetStarter().GetApplication().GetDefaultTrafficCodecKey())
	if err != nil {
		return nil, err
	}
	j.SetStatus(StateLoaded)
	return jcodec, nil
}
