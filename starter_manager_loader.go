package fiberhouse

import "errors"

// LoadProviderManagersAtLocation 按执行位点加载管理器；扩展替代管理器只替代同一位点的普通管理器。
//
// 该函数是框架对外导出的扩展点，供自定义核心启动器（CoreStarter 的第三方实现）
// 复用框架统一的「位点 provider 分派与替代」语义，无需自行复制一份等价逻辑。
// 内置的 Fiber/Gin 核心与 example_application 下的 Hertz 核心均基于本函数实现。
//
// 参数：
//   - managers   当前启动阶段传入的提供者管理器列表
//   - location   本次要处理的执行位点
//   - dependency 注入给管理器 LoadProvider 的依赖实例（通常为核心启动器自身）
//
// 返回：
//   - handled  该位点是否有管理器被实际加载
//   - replaced 是否由扩展替代组（GroupExtendReplace）接管；
//     调用方应据此跳过该位点的默认逻辑
//   - err      各管理器加载错误的聚合（errors.Join）
func LoadProviderManagersAtLocation(
	managers []IProviderManager,
	location IProviderLocation,
	dependency any,
) (handled bool, replaced bool, err error) {
	var defaults []IProviderManager
	var replacements []IProviderManager

	for _, manager := range managers {
		if manager == nil || manager.Location() == nil ||
			manager.Location().GetLocationID() != location.GetLocationID() {
			continue
		}
		if manager.Type().GetTypeID() == ProviderTypeDefault().GroupExtendReplace.GetTypeID() {
			replacements = append(replacements, manager)
			continue
		}
		defaults = append(defaults, manager)
	}

	selected := defaults
	if len(replacements) > 0 {
		selected = replacements
		replaced = true
	}
	if len(selected) == 0 {
		return false, replaced, nil
	}

	var errs []error
	for _, manager := range selected {
		_, loadErr := manager.LoadProvider(func(IProviderManager) (any, error) {
			return dependency, nil
		})
		if loadErr != nil {
			errs = append(errs, loadErr)
		}
	}
	return true, replaced, errors.Join(errs...)
}
