package jsoncodec

import (
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/component/jsoncodec"
)

type JsonJCodecGinProvider struct {
	fiberhouse.IProvider
}

// NewJsonJCodecGinProvider 创建一个新的 JSON 编解码提供者
func NewJsonJCodecGinProvider() *JsonJCodecGinProvider {
	return &JsonJCodecGinProvider{
		IProvider: fiberhouse.NewProvider().SetName("json_codec").SetTarget("gin").SetType(fiberhouse.ProviderTypeDefault().GroupJsonCodecChoose),
	}
}

// Initialize 初始化 JSON 编解码提供者
func (j *JsonJCodecGinProvider) Initialize(ctx fiberhouse.IContext, fn ...fiberhouse.ProviderInitFunc) (any, error) {
	j.Check()
	if j.Status() == fiberhouse.StateLoaded {
		return nil, nil
	}
	jcodec := jsoncodec.StdJsonDefault()
	ginJson.API = jcodec
	j.SetStatus(fiberhouse.StateLoaded)
	return jcodec, nil
}
