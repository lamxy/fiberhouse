package optioninit

import (
	"fmt"
	"github.com/lamxy/fiberhouse"
)

// CoreOptionInitPManager 框架启动器选项初始化提供者管理器
type CoreOptionInitPManager struct {
	fiberhouse.IProviderManager
}

func NewCoreOptionInitPManager(ctx fiberhouse.IContext) *CoreOptionInitPManager {
	son := &CoreOptionInitPManager{
		IProviderManager: fiberhouse.NewProviderManager(ctx).
			SetName("CoreOptionInitPManager").
			SetType(fiberhouse.ProviderTypeDefault().GroupCoreStarterOptsInitUnique).
			SetOrBindToLocation(fiberhouse.ProviderLocationDefault().LocationCoreStarterOptionInit, true). // 当前管理器绑定到核心启动器选项初始化位置点，并在该位置点执行
			BindToUniqueProvider(NewCoreOptionInitProvider()),                                             // 当前管理器唯一绑定到CoreOptionInitProvider，绑定后将无需再次初始化该提供者
	}
	// 挂载到父级管理器上
	son.MountToParent(son)
	return son
}

func (m *CoreOptionInitPManager) LoadProvider(loadFunc ...fiberhouse.ProviderLoadFunc) (any, error) {
	m.Check()
	if len(m.List()) == 0 {
		return nil, fmt.Errorf("%s, no provider found", m.Name())
	}
	return m.List()[0].Initialize(m.GetContext().(fiberhouse.IApplicationContext))
}

// MountToParent 重载挂载到父级提供者管理器
func (m *CoreOptionInitPManager) MountToParent(son ...fiberhouse.IProviderManager) fiberhouse.IProviderManager {
	if len(son) > 0 {
		m.IProviderManager.MountToParent(son[0])
		return m
	}
	m.IProviderManager.MountToParent(m)
	return m
}
