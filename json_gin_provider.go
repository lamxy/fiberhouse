package fiberhouse

import (
	ginJson "github.com/gin-gonic/gin/codec/json"
	"github.com/lamxy/fiberhouse/component/jsoncodec"
)

type JsonJCodecGinProvider struct {
	IProvider
}

// NewJsonJCodecGinProvider 创建一个新的 JSON 编解码提供者
func NewJsonJCodecGinProvider() *JsonJCodecGinProvider {
	return &JsonJCodecGinProvider{
		IProvider: NewProvider().SetName("json_codec").SetTarget("gin").SetType(ProviderTypeDefault().GroupJsonCodecChoose),
	}
}

// Initialize 初始化 JSON 编解码提供者
func (j *JsonJCodecGinProvider) Initialize(ctx IContext, fn ...ProviderInitFunc) (any, error) {
	j.Check()
	if j.Status() == StateLoaded {
		return nil, nil
	}
	jcodec := jsoncodec.StdJsonDefault()
	ginJson.API = jcodec
	j.SetStatus(StateLoaded)
	return jcodec, nil
}
