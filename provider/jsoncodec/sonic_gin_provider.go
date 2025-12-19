package jsoncodec

import (
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/lamxy/fiberhouse"
)

type SonicJCodecGinProvider struct {
	fiberhouse.IProvider
}

// NewJCodecProvider 创建一个新的 JSON 编解码提供者
func NewSonicJCodecGinProvider() *SonicJCodecGinProvider {
	return &SonicJCodecGinProvider{
		IProvider: fiberhouse.NewProvider().SetName("sonic_json_codec").SetTarget("gin").SetType(fiberhouse.ProviderTypeDefault().GroupJsonCodecChoose),
	}
}

// Initialize 初始化 JSON 编解码提供者
func (j *SonicJCodecGinProvider) Initialize(ctx fiberhouse.IContext, fn ...fiberhouse.ProviderInitFunc) (any, error) {
	j.Check()
	if j.Status() == fiberhouse.StateLoaded {
		return nil, nil
	}
	// 实现 JSON 编解码器的注册逻辑
	jcodec, err := fiberhouse.GetInstance[ginJson.Core](ctx.GetStarter().GetApplication().GetDefaultJsonCodecKey())
	if err != nil {
		return nil, err
	}
	ginJson.API = jcodec
	j.SetStatus(fiberhouse.StateLoaded)
	return jcodec, nil
}
