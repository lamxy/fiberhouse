package starter

import (
	"errors"

	"github.com/lamxy/fiberhouse"
)

// loadManagersAtLocation 按執行位點載入提供者管理器；擴展替代組管理器只替代同一位點的普通管理器。
//
// 框架的同名邏輯（starter_manager_loader.go 的 loadProviderManagersAtLocation）未匯出，
// 外部核心引擎實作無法直接復用，此處以框架的匯出 API 複刻等價語義：
//   - handled  是否有管理器在該位點被實際載入
//   - replaced 是否由擴展替代組接管（呼叫方據此跳過預設邏輯）
func loadManagersAtLocation(
	managers []fiberhouse.IProviderManager,
	location fiberhouse.IProviderLocation,
	dependency any,
) (handled bool, replaced bool, err error) {
	var defaults, replacements []fiberhouse.IProviderManager

	for _, manager := range managers {
		if manager == nil || manager.Location() == nil ||
			manager.Location().GetLocationID() != location.GetLocationID() {
			continue
		}
		if manager.Type().GetTypeID() == fiberhouse.ProviderTypeDefault().GroupExtendReplace.GetTypeID() {
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
		_, loadErr := manager.LoadProvider(func(fiberhouse.IProviderManager) (any, error) {
			return dependency, nil
		})
		if loadErr != nil {
			errs = append(errs, loadErr)
		}
	}
	return true, replaced, errors.Join(errs...)
}
