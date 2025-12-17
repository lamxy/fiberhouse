package jsoncodec

import (
	"github.com/lamxy/fiberhouse"
)

// JsonCodecManager JSON 编解码提供者管理器
type JsonCodecManager struct {
	fiberhouse.IProviderManager
}

// NewJsonCodecManager 创建一个新的 JSON 编解码管理器
func NewJsonCodecManager(ctx fiberhouse.IApplicationContext) *JsonCodecManager {
	return &JsonCodecManager{
		IProviderManager: fiberhouse.NewProviderManager(ctx),
	}
}
