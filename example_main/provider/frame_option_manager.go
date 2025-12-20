package provider

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
)

// FrameOptionInitPManager 框架启动器选项初始化提供者管理器
type FrameOptionInitPManager struct {
	fiberhouse.IProviderManager
}

func NewFrameOptionInitPManager(ctx fiberhouse.IContext) *FrameOptionInitPManager {
	son := &FrameOptionInitPManager{
		IProviderManager: fiberhouse.NewProviderManager(ctx).
			SetName("FrameOptionManager").
			SetType(fiberhouse.ProviderTypeDefault().GroupFrameStarterOptsInitUnique).
			SetOrBindToLocation(fiberhouse.ProviderLocationDefault().LocationFrameStarterOptionInit, true). // 当前管理器绑定到框架启动器选项初始化位置点，并在该位置点执行
			BindToUniqueProvider(NewFrameOptionInitProvider()),                                             // 当前管理器唯一绑定到 FrameOptionInitProvider，绑定后将无需初始化提供者
	}
	// 挂载到父级提供者管理器
	son.MountToParent(son)
	return son
}

// LoadProvider 重载加载提供者
func (m *FrameOptionInitPManager) LoadProvider(loadFunc ...fiberhouse.ProviderLoadFunc) (any, error) {
	m.Check()
	if len(m.List()) == 0 {
		return nil, fmt.Errorf("%s, no provider found", m.Name())
	}
	return m.List()[0].Initialize(m.GetContext().(fiberhouse.IApplicationContext))
}

// MountToParent 重载挂载到父级提供者管理器
func (m *FrameOptionInitPManager) MountToParent(son ...fiberhouse.IProviderManager) fiberhouse.IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
