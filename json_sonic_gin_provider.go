package fiberhouse

import (
	ginJson "github.com/gin-gonic/gin/codec/json"
)

type SonicJCodecGinProvider struct {
	IProvider
}

// NewJCodecProvider 创建一个新的 JSON 编解码提供者
func NewSonicJCodecGinProvider() *SonicJCodecGinProvider {
	return &SonicJCodecGinProvider{
		IProvider: NewProvider().SetName("sonic_json_codec").SetTarget("gin").SetType(ProviderTypeDefault().GroupJsonCodecChoose),
	}
}

// Initialize 重载初始化 JSON 编解码提供者
func (j *SonicJCodecGinProvider) Initialize(ctx IContext, fn ...ProviderInitFunc) (any, error) {
	j.Check()
	if j.Status() == StateLoaded {
		return nil, nil
	}
	// 实现 JSON 编解码器的注册逻辑
	jcodec, err := GetInstance[ginJson.Core](ctx.GetStarter().GetApplication().GetDefaultJsonCodecKey())
	if err != nil {
		return nil, err
	}
	ginJson.API = jcodec
	j.SetStatus(StateLoaded)
	return jcodec, nil
}
