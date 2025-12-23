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
			// 当前管理器绑定到框架启动器选项初始化位置点，并在该位置点执行
			SetOrBindToLocation(fiberhouse.ProviderLocationDefault().LocationFrameStarterOptionInit, true).
			// 当前管理器唯一绑定到 FrameOptionInitProvider，绑定后将无需再次初始化提供者
			BindToUniqueProvider(NewFrameOptionInitProvider()),
	}
	// 挂载子类实例到父级提供者管理器的sonManager字段上，确保多态行为正确
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

// MountToParent 重载挂载到父级提供者管理器方法
// 注意: 该方法的重载实现不是必须的，当NewFrameOptionInitPManager()内调用基类的MountToParent方法时，则无需重载该方法，二选一
func (m *FrameOptionInitPManager) MountToParent(son ...fiberhouse.IProviderManager) fiberhouse.IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son...)
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
