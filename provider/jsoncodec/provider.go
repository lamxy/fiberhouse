package jsoncodec

import (
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/lamxy/fiberhouse"
)

type SonicJCodecProvider struct {
	fiberhouse.IProvider
}

// NewJCodecProvider 创建一个新的 JSON 编解码提供者
func NewJCodecProvider() *SonicJCodecProvider {
	return &SonicJCodecProvider{
		IProvider: fiberhouse.NewProvider().SetName("sonic_json_codec").SetVersion("").SetTarget("gin").SetType(fiberhouse.ProviderTypeDefault().JsonCodec),
	}
}

// Initialize 初始化 JSON 编解码提供者
func (j *SonicJCodecProvider) Initialize(ctx fiberhouse.IContext, fn ...fiberhouse.ProviderInitFunc) (any, error) {
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
