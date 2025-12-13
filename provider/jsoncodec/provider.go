package jsoncodec

import (
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/provider"
)

type SonicJCodecProvider struct {
	provider.IProvider
}

// NewJCodecProvider 创建一个新的 JSON 编解码提供者
func NewJCodecProvider() *SonicJCodecProvider {
	return &SonicJCodecProvider{
		IProvider: provider.NewProvider().SetName("sonic_json_codec").SetVersion("").SetTarget("gin").SetType("sonic"),
	}
}

// Initialize 初始化 JSON 编解码提供者
func (j *SonicJCodecProvider) Initialize(ctx fiberhouse.IContext, fn ...provider.InitFunc) error {
	if j.Status() == provider.StateLoaded {
		return nil
	}
	// 实现 JSON 编解码器的注册逻辑
	jcodec, err := fiberhouse.GetInstance[ginJson.Core](ctx.GetStarter().GetApplication().GetDefaultJsonCodecKey())
	if err != nil {
		return err
	}
	ginJson.API = jcodec
	j.SetStatus(provider.StateLoaded)
	return nil
}
