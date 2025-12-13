package jsoncodec

import (
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/provider"
)

// JsonCodecManager JSON 编解码提供者管理器
type JsonCodecManager struct {
	provider.IManager
}

// NewJsonCodecManager 创建一个新的 JSON 编解码管理器
func NewJsonCodecManager(ctx fiberhouse.IApplicationContext) *JsonCodecManager {
	return &JsonCodecManager{
		IManager: provider.NewManager(ctx),
	}
}
